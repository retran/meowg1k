// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Session identifiers.

use std::fmt;

/// Crockford base32, which drops the letters that look like digits.
const ALPHABET: &[u8; 32] = b"0123456789ABCDEFGHJKMNPQRSTVWXYZ";

/// How many characters of the identifier a person types.
///
/// `[R-SESSION-034]`.
pub const SHORT_LEN: usize = 8;

/// The identifier of one session.
///
/// Twenty-six characters: a 48-bit millisecond timestamp followed by 80 bits
/// of randomness, in Crockford base32. Sorting the text sorts by creation
/// time, because the timestamp leads.
///
/// The short form is the **last** eight characters, which `[R-SESSION-034]`
/// requires and `[R-SESSION-030]` got wrong: a prefix of a time-sortable
/// identifier is almost entirely timestamp, so two sessions created in the
/// same period would share it.
#[derive(
    Debug, Clone, PartialEq, Eq, Hash, PartialOrd, Ord, serde::Serialize, serde::Deserialize,
)]
pub struct SessionId(String);

impl SessionId {
    /// Build an identifier from a millisecond timestamp and 80 bits of entropy.
    ///
    /// Both are arguments rather than read here, so that a test can produce a
    /// known identifier and this crate keeps its promise to perform no input
    /// or output.
    pub fn new(millis: u64, entropy: [u8; 10]) -> Self {
        let mut bits = Vec::with_capacity(16);
        bits.extend_from_slice(&millis.to_be_bytes()[2..]); // low 48 bits
        bits.extend_from_slice(&entropy);

        // 128 bits into 26 base32 characters, most significant first.
        let mut n = 0u128;
        for b in &bits {
            n = (n << 8) | u128::from(*b);
        }
        let mut out = [b'0'; 26];
        for i in (0..26).rev() {
            out[i] = ALPHABET[(n & 0x1f) as usize];
            n >>= 5;
        }
        Self(String::from_utf8_lossy(&out).into_owned())
    }

    /// Wrap text that is already an identifier.
    pub fn from_text(text: impl Into<String>) -> Self {
        Self(text.into())
    }

    /// The whole identifier.
    pub fn as_str(&self) -> &str {
        &self.0
    }

    /// The eight characters a person types, per `[R-SESSION-034]`.
    pub fn short(&self) -> &str {
        let n = self.0.len();
        &self.0[n.saturating_sub(SHORT_LEN)..]
    }
}

impl fmt::Display for SessionId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn identifiers_sort_by_creation_time() {
        let early = SessionId::new(1_700_000_000_000, [0xff; 10]);
        let late = SessionId::new(1_700_000_001_000, [0x00; 10]);
        assert!(early < late, "{early} should sort before {late}");
    }

    #[test]
    fn the_short_form_is_the_random_tail() {
        // Same millisecond, different entropy: a prefix would collide and the
        // tail must not. This is the whole reason R-SESSION-030 was withdrawn.
        let a = SessionId::new(1_700_000_000_000, [1; 10]);
        let b = SessionId::new(1_700_000_000_000, [2; 10]);
        assert_eq!(a.as_str()[..8], b.as_str()[..8], "a prefix does collide");
        assert_ne!(a.short(), b.short(), "the tail must not");
        assert_eq!(a.short().len(), SHORT_LEN);
    }
}
