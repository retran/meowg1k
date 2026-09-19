// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Turning a directory of Starlark files into a registry.

use std::cell::RefCell;
use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::Arc;

use starlark::environment::{FrozenModule, Globals, Module};
use starlark::eval::{Evaluator, FileLoader};
use starlark::syntax::{AstModule, Dialect};

use crate::declare::globals;
use crate::error::{Result, StarError, closest};
use crate::modules::{Modules, NAMES};
use crate::registry::Registry;
use crate::state::Declaring;

/// What a workspace turned into.
#[derive(Debug)]
pub struct Loaded {
    /// Everything that was declared.
    pub registry: Registry,
    /// Every file that was evaluated, in the order it was first reached.
    pub files: Vec<PathBuf>,
    /// The evaluated files, by load key.
    ///
    /// Kept because a tool handler lives in one of them and has to survive the
    /// load: the run phase looks it up here rather than holding a value that
    /// cannot outlive the evaluator that made it.
    pub modules: HashMap<String, FrozenModule>,
    /// The `@std//` table.
    pub std: Arc<Modules>,
}

/// Refuse `print`.
///
/// `[R-STAR-082]`: it would write straight through the terminal frame and
/// corrupt it, so the error names the thing to use instead rather than saying
/// only that this is not allowed.
#[derive(Debug)]
pub struct NoPrint;

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
    std: Arc<Modules>,
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
        std_module(&self.std, name)
    }

    fn local(&self, path: &str) -> Result<FrozenModule> {
        let file = self.workspace.resolve_local(path)?;
        let key = path.to_owned();

        // [R-STAR-053]: a markdown file has no Starlark in it, so it becomes a
        // module with one symbol rather than something to evaluate. Routing it
        // through `load` rather than giving it a builtin of its own means the
        // escape check, the cycle check, and the evaluate-once cache all apply
        // to it without being written twice.
        if path.ends_with(".md") {
            if let Some(module) = self.done.borrow().get(&key) {
                return Ok(module.clone());
            }
            let module = self.prompt_module(&file)?;
            self.done.borrow_mut().insert(key, module.clone());
            return Ok(module);
        }

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

    /// Read every `.md` file under `.meow/agents/`.
    ///
    /// After `meow.star`, so a markdown agent and a Starlark agent compete for
    /// a name on equal terms and `[R-STAR-031]` reports the collision either
    /// way round. Sorted, so a duplicate names the same two files whatever
    /// order the directory happens to be read in.
    fn markdown_agents(&self) -> Result<()> {
        let dir = self.workspace.config_dir().join("agents");
        let Ok(entries) = std::fs::read_dir(&dir) else {
            return Ok(());
        };

        let mut files: Vec<PathBuf> = entries
            .filter_map(std::result::Result::ok)
            .map(|e| e.path())
            .filter(|p| p.extension().is_some_and(|e| e == "md"))
            .collect();
        files.sort();

        for file in files {
            let Some(name) = crate::markdown::name_of(&file) else {
                continue;
            };
            let text = self.read(&file)?;
            self.order.borrow_mut().push(file.clone());
            let declared =
                crate::markdown::agent(&file, &name, &text, |path| self.prompt_text(path))?;
            self.state.registry_mut().add_agent(declared)?;
        }
        Ok(())
    }

    /// Wrap a markdown file as a module exporting its text.
    fn prompt_module(&self, file: &PathBuf) -> Result<FrozenModule> {
        let text = self.read(file)?;
        self.order.borrow_mut().push(file.clone());
        Module::with_temp_heap(|module| {
            let value = module.heap().alloc(text.as_str());
            module.set("text", value);
            module
                .freeze()
                .map_err(|e| StarError::Starlark(format!("{e:?}")))
        })
    }

    /// Read a `.meow/lib/*.md` prompt named by an `include`.
    ///
    /// `[R-STAR-054]`. The path goes through the same check every other load
    /// does, so an `include` cannot reach outside `.meow/` either.
    fn prompt_text(&self, path: &str) -> Result<String> {
        if !path.ends_with(".md") {
            return Err(StarError::Load {
                message: format!("`include` names a markdown file, and `{path}` is not one"),
            });
        }
        let file = self.workspace.resolve_local(path.trim_start_matches('/'))?;
        Ok(self.read(&file)?.trim().to_owned())
    }

    fn read(&self, file: &PathBuf) -> Result<String> {
        std::fs::read_to_string(file).map_err(|source| StarError::Io {
            path: file.clone(),
            source,
        })
    }

    fn evaluate(&self, file: &PathBuf, name: &str) -> Result<FrozenModule> {
        let source = self.read(file)?;
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

    let std = Arc::new(Modules::build()?);

    let loader = Loader {
        workspace,
        state: &state,
        globals: &globals,
        std: Arc::clone(&std),
        done: RefCell::new(HashMap::new()),
        stack: RefCell::new(Vec::new()),
        order: RefCell::new(Vec::new()),
    };

    loader.local("meow.star")?;
    loader.markdown_agents()?;

    let files = loader.order.into_inner();
    let modules = loader.done.into_inner();

    let registry = state.finish();
    registry.resolve()?;

    Ok(Loaded {
        registry,
        files,
        modules,
        std,
    })
}

/// Resolve an `@std//` name against the one table.
///
/// `[R-STAR-003]`: a name with no module fails listing what there is.
/// `[R-STAR-011]`: the declaration phase and the run phase call this same
/// function, so a module cannot exist in one and not the other.
fn std_module(table: &Modules, name: &str) -> Result<FrozenModule> {
    table.get(name).cloned().ok_or_else(|| StarError::Load {
        message: format!(
            "there is no module `@std//{name}`{}. Available: {}.",
            closest(name, table.names().map(String::as_str))
                .map(|c| format!(". Did you mean `@std//{c}`?"))
                .unwrap_or_default(),
            NAMES.join(", ")
        ),
    })
}

/// The loader a handler is evaluated against.
///
/// A handler's own file was loaded during the declaration phase, so nothing
/// here evaluates anything: the modules already exist and this hands them out.
/// Having one is still necessary, because a `load` inside a function body is
/// evaluated when the function runs.
#[derive(Debug)]
pub struct RuntimeLoader<'a> {
    runtime: &'a crate::run::Runtime,
}

impl<'a> RuntimeLoader<'a> {
    /// A loader over a running workspace.
    pub fn new(runtime: &'a crate::run::Runtime) -> Self {
        Self { runtime }
    }
}

impl FileLoader for RuntimeLoader<'_> {
    fn load(&self, path: &str) -> starlark::Result<FrozenModule> {
        let resolved = if let Some(name) = path.strip_prefix("@std//") {
            std_module(self.runtime.std(), name)
        } else if let Some(local) = path.strip_prefix("//") {
            self.runtime
                .files()
                .get(local)
                .cloned()
                .ok_or_else(|| StarError::Load {
                    message: format!("`//{local}` was not loaded from .meow/"),
                })
        } else {
            Err(StarError::Load {
                message: format!("`{path}` is not a load path"),
            })
        };
        resolved.map_err(|e| starlark::Error::new_other(anyhow::anyhow!("{e}")))
    }
}
