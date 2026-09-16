// SPDX-License-Identifier: MIT

//! The crate's public surface is reachable from outside the crate.

#[test]
fn version_is_semver_shaped() {
    let v = hop_top_vstar::version();
    let mut parts = v.split('-').next().unwrap().split('.');
    for _ in 0..3 {
        assert!(parts
            .next()
            .is_some_and(|p| !p.is_empty() && p.chars().all(|c| c.is_ascii_digit())));
    }
    assert!(parts.next().is_none());
}
