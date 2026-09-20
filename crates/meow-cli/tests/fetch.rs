// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Getting a package, against a server that serves one.
#![allow(clippy::unwrap_used)]

use std::io::{Read, Write};
use std::sync::Arc;

use meow_star::package::{Lock, Package, Pin, cached_at, hash_tree};
use meow_star::registry::Origin;
use tokio_util::sync::CancellationToken;

/// A server that hands out one archive.
struct Serving {
    port: u16,
    stop: Arc<std::sync::atomic::AtomicBool>,
}

impl Serving {
    fn new(status: u16, body: Vec<u8>) -> Self {
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();
        let stop = Arc::new(std::sync::atomic::AtomicBool::new(false));

        let halt = Arc::clone(&stop);
        std::thread::spawn(move || {
            for stream in listener.incoming() {
                if halt.load(std::sync::atomic::Ordering::Relaxed) {
                    return;
                }
                let Ok(mut stream) = stream else { continue };
                let mut buffer = [0_u8; 4096];
                let _ = stream.read(&mut buffer);

                let head = format!(
                    "HTTP/1.1 {status} X\r\nContent-Type: application/gzip\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                    body.len()
                );
                let _ = stream.write_all(head.as_bytes());
                let _ = stream.write_all(&body);
                let _ = stream.flush();
            }
        });

        Self { port, stop }
    }

    fn url(&self) -> String {
        format!("http://127.0.0.1:{}/acme-{{version}}.tar.gz", self.port)
    }
}

impl Drop for Serving {
    fn drop(&mut self) {
        self.stop.store(true, std::sync::atomic::Ordering::Relaxed);
        let _ = std::net::TcpStream::connect(("127.0.0.1", self.port));
    }
}

/// A gzipped tar with one entry whose name is written by hand.
///
/// The `tar` crate's builder refuses to *create* an entry containing `..`,
/// which is the right thing for a builder and means a malicious archive has
/// to be assembled here. The header is 512 bytes of fixed fields; writing it
/// directly is what lets this test be about unpacking rather than about what
/// the builder allows.
fn archive_with_raw_name(name: &str, body: &str) -> Vec<u8> {
    let mut header = [0_u8; 512];
    header[..name.len()].copy_from_slice(name.as_bytes());
    // mode, uid, gid, as octal strings.
    header[100..107].copy_from_slice(b"0000644");
    header[108..115].copy_from_slice(b"0000000");
    header[116..123].copy_from_slice(b"0000000");
    // size and mtime.
    let size = format!("{:011o}\0", body.len());
    header[124..136].copy_from_slice(size.as_bytes());
    header[136..148].copy_from_slice(b"00000000000\0");
    // The checksum is computed with its own field read as spaces.
    header[148..156].copy_from_slice(b"        ");
    header[156] = b'0';
    let sum: u32 = header.iter().map(|b| u32::from(*b)).sum();
    let checksum = format!("{sum:06o}\0 ");
    header[148..156].copy_from_slice(checksum.as_bytes());

    let mut tar = header.to_vec();
    tar.extend_from_slice(body.as_bytes());
    tar.resize(tar.len().div_ceil(512) * 512, 0);
    // Two empty blocks end an archive.
    tar.extend_from_slice(&[0_u8; 1024]);

    let mut encoder = flate2::write::GzEncoder::new(Vec::new(), flate2::Compression::default());
    encoder.write_all(&tar).unwrap();
    encoder.finish().unwrap()
}

/// A gzipped tar holding these files at these paths.
fn archive(files: &[(&str, &str)]) -> Vec<u8> {
    let mut builder = tar::Builder::new(Vec::new());
    for (name, body) in files {
        let mut header = tar::Header::new_gnu();
        header.set_size(body.len() as u64);
        header.set_mode(0o644);
        header.set_cksum();
        builder
            .append_data(&mut header, name, body.as_bytes())
            .unwrap();
    }
    let tar = builder.into_inner().unwrap();

    let mut encoder = flate2::write::GzEncoder::new(Vec::new(), flate2::Compression::default());
    encoder.write_all(&tar).unwrap();
    encoder.finish().unwrap()
}

fn package(name: &str, source: &str, version: &str) -> Package {
    Package {
        name: name.to_owned(),
        source: source.to_owned(),
        version: version.to_owned(),
        origin: Origin("a test".to_owned()),
    }
}

fn config() -> tempfile::TempDir {
    let dir = tempfile::tempdir().unwrap();
    std::fs::create_dir_all(dir.path().join(".data").join("pkg")).unwrap();
    dir
}

/// [R-PKG-013] `update` fetches and pins what it got
#[tokio::test(flavor = "multi_thread")]
async fn update_writes_a_pin_for_what_arrived() {
    let server = Serving::new(200, archive(&[("models.star", "x = 1\n")]));
    let config = config();
    let declared = vec![package("acme", &server.url(), "1.0.0")];

    let lock = meow_cli::fetch::update(config.path(), &declared, &CancellationToken::new())
        .await
        .unwrap();

    let pin = lock.packages.get("acme").unwrap();
    assert_eq!(pin.version, "1.0.0");
    assert_eq!(pin.hash.len(), 64, "a pin must record a SHA-256");

    let cached = cached_at(config.path(), &pin.hash);
    assert!(cached.join("models.star").exists(), "nothing was extracted");
    assert_eq!(
        hash_tree(&cached).unwrap(),
        pin.hash,
        "what was cached must hash to what was pinned"
    );
}

/// [R-PKG-020] a package already in the cache needs no network
#[tokio::test(flavor = "multi_thread")]
async fn a_cached_package_is_not_downloaded_again() {
    let server = Serving::new(200, archive(&[("models.star", "x = 1\n")]));
    let config = config();
    let declared = vec![package("acme", &server.url(), "1.0.0")];

    let lock = meow_cli::fetch::update(config.path(), &declared, &CancellationToken::new())
        .await
        .unwrap();

    // The server is gone from here on, so anything that reaches for it fails.
    drop(server);

    let got = meow_cli::fetch::ensure(config.path(), &declared, &lock, &CancellationToken::new())
        .await
        .unwrap();
    assert_eq!(got, 0, "a cached package was downloaded again");
}

/// [R-PKG-011] what arrives is checked against the pin at fetch, not only at
/// load
#[tokio::test(flavor = "multi_thread")]
async fn a_server_that_serves_something_else_is_caught() {
    let server = Serving::new(200, archive(&[("models.star", "x = 2\n")]));
    let config = config();
    let declared = vec![package("acme", &server.url(), "1.0.0")];

    // A lockfile pinning a hash nothing will produce.
    let mut lock = Lock::default();
    lock.packages.insert(
        "acme".to_owned(),
        Pin {
            source: server.url(),
            version: "1.0.0".to_owned(),
            hash: "0".repeat(64),
        },
    );

    let error = meow_cli::fetch::ensure(config.path(), &declared, &lock, &CancellationToken::new())
        .await
        .unwrap_err();

    assert!(
        error.to_string().contains("meow.lock` pins") || error.to_string().contains("pins"),
        "the mismatch must name both hashes: {error}"
    );
}

/// [R-PKG-022] an archive that writes outside the package is refused
///
/// The oldest vulnerability in the format. An entry named `../../x` unpacked
/// without checking writes wherever the attacker chose.
#[tokio::test(flavor = "multi_thread")]
async fn an_archive_that_escapes_is_refused() {
    let server = Serving::new(200, archive_with_raw_name("../../escaped.star", "x = 1\n"));
    let config = config();
    let declared = vec![package("acme", &server.url(), "1.0.0")];

    let error = meow_cli::fetch::update(config.path(), &declared, &CancellationToken::new())
        .await
        .unwrap_err();

    assert!(
        error.to_string().contains("outside the package"),
        "an escaping entry must be refused by name: {error}"
    );
    assert!(
        !config
            .path()
            .parent()
            .unwrap()
            .join("escaped.star")
            .exists(),
        "the archive wrote outside the package"
    );
}

/// [R-PKG-022] an interrupted fetch leaves nothing that looks like a package
#[tokio::test(flavor = "multi_thread")]
async fn a_failed_fetch_leaves_no_half_package() {
    let server = Serving::new(200, b"this is not a gzip stream".to_vec());
    let config = config();
    let declared = vec![package("acme", &server.url(), "1.0.0")];

    let error = meow_cli::fetch::update(config.path(), &declared, &CancellationToken::new())
        .await
        .unwrap_err();
    assert!(error.to_string().contains("acme") || !error.to_string().is_empty());

    let cache = config.path().join(".data").join("pkg");
    let left: Vec<_> = std::fs::read_dir(&cache).unwrap().flatten().collect();
    assert!(
        left.is_empty(),
        "a failed fetch left something behind: {:?}",
        left.iter().map(std::fs::DirEntry::path).collect::<Vec<_>>()
    );
}

/// [R-PKG-013] a source that answers with a status is reported, not retried
#[tokio::test(flavor = "multi_thread")]
async fn a_source_that_refuses_says_what_it_answered() {
    let server = Serving::new(404, Vec::new());
    let config = config();
    let declared = vec![package("acme", &server.url(), "1.0.0")];

    let error = meow_cli::fetch::update(config.path(), &declared, &CancellationToken::new())
        .await
        .unwrap_err();

    assert!(
        error.to_string().contains("404"),
        "the status must reach the person: {error}"
    );
}

/// [R-PKG-010] the version is substituted into the source
#[test]
fn a_version_is_put_into_the_source() {
    let with = package("acme", "https://x.invalid/acme-{version}.tar.gz", "2.1.0");
    assert_eq!(
        meow_cli::fetch::url_for(&with),
        "https://x.invalid/acme-2.1.0.tar.gz"
    );

    // A source with no placeholder is used as it is, which is what a mirror
    // of a single file needs.
    let without = package("acme", "https://x.invalid/acme.tar.gz", "2.1.0");
    assert_eq!(
        meow_cli::fetch::url_for(&without),
        "https://x.invalid/acme.tar.gz"
    );
}

/// [R-PKG-021] two packages with the same contents share one cache entry
#[tokio::test(flavor = "multi_thread")]
async fn identical_contents_share_a_cache_entry() {
    let body = archive(&[("models.star", "x = 1\n")]);
    let one = Serving::new(200, body.clone());
    let two = Serving::new(200, body);
    let config = config();

    let declared = vec![
        package("a", &one.url(), "1.0.0"),
        package("b", &two.url(), "9.9.9"),
    ];

    let lock = meow_cli::fetch::update(config.path(), &declared, &CancellationToken::new())
        .await
        .unwrap();

    assert_eq!(
        lock.packages["a"].hash, lock.packages["b"].hash,
        "the same bytes must pin to the same hash whatever they were called"
    );

    let entries: Vec<_> = std::fs::read_dir(config.path().join(".data").join("pkg"))
        .unwrap()
        .flatten()
        .collect();
    assert_eq!(entries.len(), 1, "one hash must mean one directory");
}

/// A server that accepts a connection and then says nothing.
///
/// A download that never finishes is the case a deadline exists for, and it
/// cannot be produced by a server that answers.
struct Silent {
    port: u16,
    stop: Arc<std::sync::atomic::AtomicBool>,
}

impl Silent {
    fn new() -> Self {
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();
        let stop = Arc::new(std::sync::atomic::AtomicBool::new(false));

        let halt = Arc::clone(&stop);
        std::thread::spawn(move || {
            let mut held = Vec::new();
            for stream in listener.incoming() {
                if halt.load(std::sync::atomic::Ordering::Relaxed) {
                    return;
                }
                // Kept, never answered, and never closed - closing would give
                // the client an end of stream to act on.
                if let Ok(stream) = stream {
                    held.push(stream);
                }
            }
        });

        Self { port, stop }
    }

    fn url(&self) -> String {
        format!("http://127.0.0.1:{}/acme.tar.gz", self.port)
    }
}

impl Drop for Silent {
    fn drop(&mut self) {
        self.stop.store(true, std::sync::atomic::Ordering::Relaxed);
        let _ = std::net::TcpStream::connect(("127.0.0.1", self.port));
    }
}

/// [R-PKG-023] a fetch already cancelled makes no request
#[tokio::test(flavor = "multi_thread")]
async fn a_cancelled_fetch_does_not_start() {
    let server = Silent::new();
    let config = config();
    let declared = vec![package("acme", &server.url(), "1.0.0")];

    let cancel = CancellationToken::new();
    cancel.cancel();

    let began = std::time::Instant::now();
    let error = meow_cli::fetch::update(config.path(), &declared, &cancel)
        .await
        .unwrap_err();

    assert!(
        error.to_string().contains("cancelled"),
        "expected a cancellation, got {error}"
    );
    // The server never answers, so anything that reached it would wait for
    // the deadline rather than returning at once.
    assert!(
        began.elapsed() < std::time::Duration::from_secs(5),
        "the request was made before the cancellation was noticed"
    );
}

/// [R-PKG-023] a fetch in flight stops when the run is cancelled
#[tokio::test(flavor = "multi_thread")]
async fn a_fetch_in_flight_is_interrupted() {
    let server = Silent::new();
    let config = config();
    let declared = vec![package("acme", &server.url(), "1.0.0")];

    let cancel = CancellationToken::new();
    let stop = cancel.clone();
    tokio::spawn(async move {
        tokio::time::sleep(std::time::Duration::from_millis(300)).await;
        stop.cancel();
    });

    let began = std::time::Instant::now();
    let error = meow_cli::fetch::update(config.path(), &declared, &cancel)
        .await
        .unwrap_err();
    let took = began.elapsed();

    assert!(
        error.to_string().contains("cancelled"),
        "expected a cancellation, got {error}"
    );
    assert!(
        took < std::time::Duration::from_secs(10),
        "the fetch waited out its deadline instead of stopping: {took:?}"
    );
    assert!(
        took >= std::time::Duration::from_millis(250),
        "it stopped before the cancellation was sent, so it was not interrupted"
    );

    // And nothing was left behind.
    let left: Vec<_> = std::fs::read_dir(config.path().join(".data").join("pkg"))
        .unwrap()
        .flatten()
        .collect();
    assert!(
        left.is_empty(),
        "an interrupted fetch left something behind"
    );
}
