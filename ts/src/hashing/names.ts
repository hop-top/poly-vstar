// SPDX-License-Identifier: MIT

// The hash property name, alone in its own module.
//
// `canonical` needs it for the rule-7 exclusion and `hashing` needs it
// for everything else; `hashing` imports `canonical`. Putting the
// string here breaks the cycle without hoisting it to the package root,
// where the API mapping is explicit that it must not live.

/**
 * The property carrying a component's canonical-form hash.
 *
 * The constant belongs to the `hashing` package and stays there in
 * every port: the string is meaningful only in company with the hash
 * functions that write and read it, and the canonical layer's rule-7
 * exclusion of this property is a hashing concern.
 */
export const X_VSTAR_HASH_PROPERTY = "X-VSTAR-HASH";
