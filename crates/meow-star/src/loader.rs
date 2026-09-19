// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Turning a directory of Starlark files into a registry.

use std::cell::RefCell;
use std::collections::HashMap;
use std::path::PathBuf;

use starlark::environment::{FrozenModule, Globals, GlobalsBuilder, Module};
use starlark::eval::{Evaluator, FileLoader};
use starlark::starlark_module;
use starlark::syntax::{AstModule, Dialect};
use starlark::values::none::NoneOr;

use crate::declare::{Declaring, globals};
use crate::error::{Result, StarError, closest};
use crate::registry::Registry;

/// The runtime modules a declaration file may load.
///
/// `[R-STAR-084]`: `@std//env` and nothing else. Credentials are resolved in
/// a declaration file, so forbidding the environment outright would make the
/// normal configuration impossible; every other module stays out, so loading
/// `.meow/` cannot have consequences.
const DECLARING_MODULES: &[&str] = &["env"];

/// Every runtime module, whether or not it is available yet.
///
/// Listed here so `[R-STAR-003]` can tell "no such module" from "not during
/// declaration", and so `[R-STAR-091]` has something to suggest from.
const ALL_MODULES: &[&str] = &[
    "env", "exec", "fs", "http", "json", "path", "re", "text", "time",
];

/// What a workspace turned into.
#[derive(Debug)]
pub struct Loaded {
    /// Everything that was declared.
    pub registry: Registry,
    /// Every file that was evaluated, in the order it was first reached.
    pub files: Vec<PathBuf>,
}

/// `@std//env`, the one module a declaration file may use.
#[starlark_module]
fn env_module(builder: &mut GlobalsBuilder) {
    /// Read an environment variable.
    fn get(
        #[starlark(require = pos)] name: String,
        #[starlark(require = pos)] default: Option<String>,
    ) -> starlark::Result<NoneOr<String>> {
        Ok(match std::env::var(&name).ok().or(default) {
            Some(value) => NoneOr::Other(value),
            None => NoneOr::None,
        })
    }
}

/// Refuse `print`.
///
/// `[R-STAR-082]`: it would write straight through the terminal frame and
/// corrupt it, so the error names the thing to use instead rather than saying
/// only that this is not allowed.
struct NoPrint;

impl starlark::PrintHandler for NoPrint {
    fn println(&self, _text: &str) -> starlark::Result<()> {
        Err(starlark::Error::new_other(anyhow::anyhow!(
            "`print` writes through the terminal frame and corrupts it. Use `ctx.out` instead."
        )))
    }
}

/// Loads `.meow/`, once per invocation.
struct Loader<'a> {
    workspace: &'a crate::Workspace,
    state: &'a Declaring,
    globals: &'a Globals,
    std_env: FrozenModule,
    /// `[R-STAR-007]`: a file evaluated once, however many times it is named.
    done: RefCell<HashMap<String, FrozenModule>>,
    /// `[R-STAR-006]`: what is being evaluated right now, innermost last.
    stack: RefCell<Vec<String>>,
    order: RefCell<Vec<PathBuf>>,
}

impl FileLoader for Loader<'_> {
    fn load(&self, path: &str) -> starlark::Result<FrozenModule> {
        self.resolve(path)
            .map_err(|e| starlark::Error::new_other(anyhow::anyhow!("{e}")))
    }
}

impl Loader<'_> {
    fn resolve(&self, path: &str) -> Result<FrozenModule> {
        if let Some(name) = path.strip_prefix("@std//") {
            return self.std_module(name);
        }
        // [R-STAR-005]: checked before `//`, so `@acme//lib.star` is never
        // read as a path relative to `.meow/`. Silently treating it as local
        // would find the wrong file on one machine and no file on another.
        if let Some(rest) = path.strip_prefix('@') {
            let pkg = rest.split("//").next().unwrap_or(rest);
            return Err(StarError::Load {
                message: format!(
                    "`{path}` names package `{pkg}`, and packages are not implemented. Use `//<path>` for a file in .meow/."
                ),
            });
        }
        if let Some(local) = path.strip_prefix("//") {
            return self.local(local);
        }
        Err(StarError::Load {
            message: format!(
                "`{path}` is not a load path. Use `@std//<module>` for a runtime module or `//<path>` for a file in .meow/."
            ),
        })
    }

    fn std_module(&self, name: &str) -> Result<FrozenModule> {
        if DECLARING_MODULES.contains(&name) {
            return Ok(self.std_env.clone());
        }
        if ALL_MODULES.contains(&name) {
            // [R-STAR-083] follows from this: a declaration file cannot write
            // a file, run a command, or make a request, because the modules
            // that could are not reachable from here.
            return Err(StarError::ModuleUnavailable {
                module: format!("@std//{name}"),
            });
        }
        Err(StarError::Load {
            message: format!(
                "there is no module `@std//{name}`{}. Available: {}.",
                closest(name, ALL_MODULES.iter().copied())
                    .map(|c| format!(". Did you mean `@std//{c}`?"))
                    .unwrap_or_default(),
                ALL_MODULES.join(", ")
            ),
        })
    }

    fn local(&self, path: &str) -> Result<FrozenModule> {
        let file = self.workspace.resolve_local(path)?;
        let key = path.to_owned();

        if let Some(module) = self.done.borrow().get(&key) {
            return Ok(module.clone());
        }

        // [R-STAR-006]: the ring is reported in the order it was walked,
        // because "circular import" without the ring is a puzzle.
        if let Some(at) = self.stack.borrow().iter().position(|p| p == &key) {
            let mut cycle: Vec<String> = self.stack.borrow()[at..].to_vec();
            cycle.push(key);
            return Err(StarError::Cycle { cycle });
        }

        self.stack.borrow_mut().push(key.clone());
        let result = self.evaluate(&file, &key);
        self.stack.borrow_mut().pop();

        let module = result?;
        self.done.borrow_mut().insert(key, module.clone());
        Ok(module)
    }

    fn evaluate(&self, file: &PathBuf, name: &str) -> Result<FrozenModule> {
        let source = std::fs::read_to_string(file).map_err(|source| StarError::Io {
            path: file.clone(),
            source,
        })?;
        self.order.borrow_mut().push(file.clone());

        let ast = AstModule::parse(name, source, &Dialect::Extended)
            .map_err(|e| StarError::Starlark(format!("{e}")))?;

        let previous = self.state.entering(name);

        // [R-STAR-080]: one evaluator per file, and it never leaves this
        // closure. The heap it allocates on is scoped to the call, so nothing
        // it touched can outlive the thread that made it.
        let outcome = Module::with_temp_heap(|module| {
            let mut eval = Evaluator::new(&module);
            eval.extra = Some(self.state);
            eval.set_loader(self);
            eval.set_print_handler(&NoPrint);

            let result = eval.eval_module(ast, self.globals);
            drop(eval);

            result
                .map_err(|e| StarError::Starlark(format!("{e}")))
                .and_then(|_| {
                    module
                        .freeze()
                        .map_err(|e| StarError::Starlark(format!("{e:?}")))
                })
        });

        self.state.entering(&previous);
        outcome
    }
}

/// Evaluate a workspace and return what it declared.
///
/// Satisfies `[R-STAR-007]` through the module cache, `[R-STAR-032]` by
/// resolving references only after every file has been evaluated, and
/// `[R-STAR-080]` by owning the evaluators for the whole call and letting none
/// of them outlive it.
///
/// # Errors
///
/// Anything in [`StarError`]. A Starlark failure keeps the diagnostic the
/// evaluator produced, which is what carries the file, line, and column
/// `[R-STAR-090]` asks for.
pub fn load(workspace: &crate::Workspace) -> Result<Loaded> {
    let state = Declaring::new();
    let globals = globals();

    let built = GlobalsBuilder::new().with(env_module).build();
    let std_env = Module::with_temp_heap(|module| {
        module.frozen_heap().add_reference(built.heap());
        for (name, value) in built.iter() {
            module.set(name, value.to_value());
        }
        module
            .freeze()
            .map_err(|e| StarError::Starlark(format!("{e:?}")))
    })?;

    let loader = Loader {
        workspace,
        state: &state,
        globals: &globals,
        std_env,
        done: RefCell::new(HashMap::new()),
        stack: RefCell::new(Vec::new()),
        order: RefCell::new(Vec::new()),
    };

    loader.local("meow.star")?;
    let files = loader.order.into_inner();

    let registry = state.finish();
    registry.resolve()?;

    Ok(Loaded { registry, files })
}
