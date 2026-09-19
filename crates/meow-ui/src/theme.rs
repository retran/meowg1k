// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Colour, and what to do when there is none.

use ratatui::style::{Color, Modifier, Style};

/// How much colour a terminal can show.
///
/// `[R-TUI-091]`: detected and quantised rather than dropped. A 16-colour
/// terminal getting grey text is a worse outcome than a 16-colour terminal
/// getting the nearest of sixteen colours, and both are better than every
/// terminal getting none because one of them was old.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Depth {
    /// No colour at all.
    None,
    /// The original sixteen.
    Ansi16,
    /// The xterm cube.
    Ansi256,
    /// Twenty-four bit.
    True,
}

impl Depth {
    /// Read the depth out of an environment.
    ///
    /// Takes the variables rather than reading them, so a test can state the
    /// environment it means instead of mutating the process.
    ///
    /// `[R-TUI-090]`: `NO_COLOR` wins over everything, including a theme that
    /// asks for colour. Its presence is what counts, not its value, which is
    /// what the convention says.
    pub fn detect(no_color: bool, colorterm: Option<&str>, term: Option<&str>) -> Self {
        if no_color {
            return Self::None;
        }
        if matches!(colorterm, Some("truecolor" | "24bit")) {
            return Self::True;
        }
        match term {
            None | Some("dumb") => Self::None,
            Some(term) if term.contains("256") || term.contains("direct") => Self::Ansi256,
            Some(_) => Self::Ansi16,
        }
    }
}

/// Whether the terminal can be shown to draw boxes and spinners.
///
/// `[R-TUI-093]`: shown to support, not assumed to. A terminal that says
/// nothing gets the ASCII fallback, because a row of replacement characters is
/// worse than a row of dashes.
pub fn unicode(lang: Option<&str>, term: Option<&str>) -> bool {
    if matches!(term, None | Some("dumb")) {
        return false;
    }
    lang.is_some_and(|l| {
        let upper = l.to_ascii_uppercase();
        upper.contains("UTF-8") || upper.contains("UTF8")
    })
}

/// What a renderer draws with.
#[derive(Debug, Clone, Copy)]
pub struct Theme {
    depth: Depth,
    unicode: bool,
}

impl Default for Theme {
    fn default() -> Self {
        Self::ascii()
    }
}

impl Theme {
    /// A theme for a terminal with the given capabilities.
    pub fn new(depth: Depth, unicode: bool) -> Self {
        Self { depth, unicode }
    }

    /// No colour, no box drawing: what a pipe and a `dumb` terminal get.
    pub fn ascii() -> Self {
        Self {
            depth: Depth::None,
            unicode: false,
        }
    }

    /// How much colour this theme may use.
    pub fn depth(&self) -> Depth {
        self.depth
    }

    /// Whether box drawing and spinner characters are safe.
    pub fn unicode(&self) -> bool {
        self.unicode
    }

    /// The style for a role, quantised to what the terminal can show.
    pub fn style(&self, role: Role) -> Style {
        if self.depth == Depth::None {
            return match role {
                Role::Emphasis => Style::default().add_modifier(Modifier::BOLD),
                Role::Muted => Style::default().add_modifier(Modifier::DIM),
                _ => Style::default(),
            };
        }
        Style::default().fg(self.quantise(role.colour()))
    }

    /// Bring a colour down to what the terminal has.
    fn quantise(&self, rgb: (u8, u8, u8)) -> Color {
        match self.depth {
            Depth::None => Color::Reset,
            Depth::True => Color::Rgb(rgb.0, rgb.1, rgb.2),
            Depth::Ansi256 => Color::Indexed(cube(rgb)),
            Depth::Ansi16 => Color::Indexed(nearest_ansi(rgb)),
        }
    }

    /// The tree character before a transcript line.
    pub fn branch(&self) -> &'static str {
        if self.unicode { "├─" } else { "|-" }
    }

    /// The tree character before the last line.
    pub fn last_branch(&self) -> &'static str {
        if self.unicode { "└─" } else { "\\-" }
    }

    /// One frame of the spinner, or a textual stand-in.
    pub fn spinner(&self, frame: usize) -> &'static str {
        const BRAILLE: [&str; 8] = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧"];
        const ASCII: [&str; 4] = ["-", "\\", "|", "/"];
        if self.unicode {
            BRAILLE[frame % BRAILLE.len()]
        } else {
            ASCII[frame % ASCII.len()]
        }
    }
}

/// What a piece of text means.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Role {
    /// Ordinary output.
    Plain,
    /// An aside.
    Muted,
    /// A heading or a step label.
    Emphasis,
    /// Something the reader should act on.
    Warning,
    /// Something that went wrong.
    Failure,
    /// Something that went right.
    Success,
}

impl Role {
    fn colour(self) -> (u8, u8, u8) {
        match self {
            Self::Plain => (205, 214, 244),
            Self::Muted => (108, 112, 134),
            Self::Emphasis => (137, 180, 250),
            Self::Warning => (249, 226, 175),
            Self::Failure => (243, 139, 168),
            Self::Success => (166, 227, 161),
        }
    }

    /// The word that carries the meaning when colour cannot.
    ///
    /// `[R-TUI-092]`: colour is never the only carrier, which is also what
    /// makes the plain renderer a faithful downgrade rather than a lossy one.
    pub fn sigil(self) -> &'static str {
        match self {
            Self::Plain | Self::Emphasis => "",
            Self::Muted => "note: ",
            Self::Warning => "warning: ",
            Self::Failure => "error: ",
            Self::Success => "ok: ",
        }
    }
}

/// The xterm 256 index nearest an RGB triple.
fn cube(rgb: (u8, u8, u8)) -> u8 {
    let level = |v: u8| -> u8 {
        // The cube's six levels are 0, 95, 135, 175, 215, 255.
        const LEVELS: [u8; 6] = [0, 95, 135, 175, 215, 255];
        let mut best = 0;
        let mut distance = u16::MAX;
        for (i, candidate) in LEVELS.iter().enumerate() {
            let d = u16::from(v.abs_diff(*candidate));
            if d < distance {
                distance = d;
                best = u8::try_from(i).unwrap_or(0);
            }
        }
        best
    };
    16 + 36 * level(rgb.0) + 6 * level(rgb.1) + level(rgb.2)
}

/// The nearest of the original sixteen.
fn nearest_ansi(rgb: (u8, u8, u8)) -> u8 {
    const PALETTE: [(u8, u8, u8); 16] = [
        (0, 0, 0),
        (128, 0, 0),
        (0, 128, 0),
        (128, 128, 0),
        (0, 0, 128),
        (128, 0, 128),
        (0, 128, 128),
        (192, 192, 192),
        (128, 128, 128),
        (255, 0, 0),
        (0, 255, 0),
        (255, 255, 0),
        (0, 0, 255),
        (255, 0, 255),
        (0, 255, 255),
        (255, 255, 255),
    ];

    let mut best = 7;
    let mut distance = u32::MAX;
    for (i, candidate) in PALETTE.iter().enumerate() {
        let d =
            squared(rgb.0, candidate.0) + squared(rgb.1, candidate.1) + squared(rgb.2, candidate.2);
        if d < distance {
            distance = d;
            best = u8::try_from(i).unwrap_or(7);
        }
    }
    best
}

fn squared(a: u8, b: u8) -> u32 {
    let d = u32::from(a.abs_diff(b));
    d * d
}
