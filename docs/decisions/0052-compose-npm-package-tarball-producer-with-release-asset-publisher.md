---
parent: Decisions
nav_order: 52
status: accepted
date: 12026-07-07
decision-makers: Yunseo Kim
---

# Compose the npm Package Tarball Producer with the GitHub Release Asset Publisher

## Context and Problem Statement

ADR 0049 redefined GitHub Release asset handling as publication rather than artifact production. ADR
0050 then required the publisher profile to accept a producer-to-publisher handoff that binds
artifact bytes, final release asset name, expected digest, and producer-generated SLSA provenance.
ADR 0051 requires the publisher to redistribute the verified producer provenance bundle as an
unchanged GitHub Release asset sidecar.

Those decisions intentionally keep the GitHub Release asset publisher from becoming a generic build
profile. They also leave an initial composition question: which producer profile should first feed
the publisher with source-to-artifact provenance and final artifact bytes?

The project already has a detailed initial producer candidate: the JS/TS npm package profile. That
profile produces an npm package tarball, generates Windlass-owned SLSA provenance for that tarball,
and returns package identity plus tarball digest outputs. The same tarball can be useful as a GitHub
Release asset for projects that want a release-page copy in addition to npm registry publication.

Should the first GitHub Release asset producer composition use the JS/TS npm package profile's
published package tarball, introduce a generic release asset builder, wait for a Go binary or
archive producer profile, or leave producer composition unspecified until multiple producer profiles
exist?

## Decision Drivers

- Preserve ADR 0049's boundary that GitHub Release asset publication does not build or mint
  source-to-artifact provenance in the default production path.
- Reuse the already-specified JS/TS npm package producer semantics instead of inventing a generic
  artifact build model.
- Provide a concrete first end-to-end composition for projects that publish npm packages and also
  attach the produced package tarball to a GitHub Release.
- Keep the GitHub Release asset publisher generic enough to compose with future producer profiles,
  including Go archives, container images, SBOM producers, checksum producers, or other ecosystem
  profiles.
- Require the producer provenance subject to match the final GitHub Release asset name and digest,
  as required by ADR 0050, without adding a publisher-owned rename predicate.
- Avoid coupling the publisher's public contract, verification model, or implementation internals to
  npm-specific fields that belong to the JS/TS npm package producer profile.
- Keep future producer profile additions possible without redesigning the publisher contract.

## Considered Options

- Use the JS/TS npm package profile's npm package tarball as the initial producer composition for
  the GitHub Release asset publisher.
- Create a generic release asset builder profile before composing any ecosystem producer.
- Wait for a Go binary or archive producer profile before defining a first publisher composition.
- Leave the initial producer composition unspecified until several producer profiles exist.

## Decision Outcome

Chosen option: "Use the JS/TS npm package profile's npm package tarball as the initial producer
composition for the GitHub Release asset publisher", because it provides the first concrete
producer-to-publisher path while preserving the publisher as an abstract verifier and distributor
for producer-owned artifacts.

The first production GitHub Release asset publisher composition should use the npm package tarball
produced by the JS/TS npm package profile as the primary release asset bytes. The JS/TS npm package
profile remains the source-to-artifact producer: it owns package-manager execution, package packing,
npm package identity, tarball digest calculation, Windlass-generated SLSA provenance, npm registry
publication, and npm-specific verification expectations.

The GitHub Release asset publisher should receive the npm tarball through the producer-to-publisher
handoff contract selected by ADR 0050. The handoff should expose the artifact handle, expected
SHA-256 digest, final GitHub Release asset name, release tag, producer provenance bundle or locator,
producer `builder.id`, producer `buildType`, producer subject name, producer subject digest, source
identity, and producer-native provenance locators using the generic handoff semantics. The publisher
must treat npm-specific fields as producer policy inputs or opaque producer metadata unless the
generic handoff contract explicitly requires them.

The publisher must verify that the npm tarball bytes it will upload match the expected digest, that
the producer provenance signature and trusted producer policy verify, and that the producer
provenance subject name and digest match the final GitHub Release asset name and bytes. It should
then upload the tarball as the primary GitHub Release asset and redistribute the unchanged producer
provenance bundle as the sidecar selected by ADR 0051.

This composition does not make the GitHub Release asset publisher an npm-specific workflow. The
publisher's architecture specification, inputs, verification phases, failure behavior, and outputs
should be expressed in producer-neutral terms: artifact handle, expected digest, final asset name,
release target, producer provenance, trusted producer policy, native provenance locators, sidecar
publication, and linked-artifact options. npm package name, package version, registry URL, dist-tag,
package URL, npm integrity values, and package-manager metadata remain JS/TS npm package producer
fields unless a generic publisher rule explicitly needs a normalized value.

Future producer profiles should be able to compose with the same publisher without changing the
publisher's trust boundary. Adding producers such as Go binary/archive, container image, checksum,
SBOM, or other ecosystem profiles should require those producers to satisfy the same handoff
contract, not require the publisher to learn the full semantics of each producer ecosystem. If a
future producer needs a materially different handoff, subject-name mapping, publication evidence, or
trust model, that change should be recorded by a future ADR and should preserve a clear distinction
between producer-owned source-to-artifact provenance and publisher-owned distribution behavior.

The initial npm tarball composition should not support generic raw file upload, arbitrary caller
artifacts, unpublished package tarballs without accepted producer provenance, or publisher-generated
SLSA provenance. Those remain outside the production publisher path unless a future ADR defines a
distinct producer profile, lower-assurance workflow, or publication evidence model.

### Consequences

- Good, because the project gets a concrete first release asset composition without reintroducing a
  generic release asset builder.
- Good, because the JS/TS npm package profile's existing package identity, tarball digest, and
  provenance decisions become immediately useful for GitHub Release distribution.
- Good, because the publisher remains a verifier and distributor rather than an npm package profile
  extension.
- Good, because future producer profiles can target the same publisher handoff contract.
- Good, because the architecture avoids treating raw caller-provided files as production release
  assets without producer provenance.
- Neutral, because the first composition primarily serves projects that publish npm packages and
  want a GitHub Release copy of the package tarball.
- Neutral, because the JS/TS npm package profile may need to expose handoff outputs that are more
  precise than ordinary public workflow outputs.
- Bad, because projects that need Go binaries, generic archives, containers, SBOM-only assets, or
  other release asset types still need later producer profiles.
- Bad, because requiring the producer subject name to match the final GitHub Release asset name may
  force npm tarball naming to be decided before provenance signing.

### Confirmation

This decision is confirmed when architecture specifications, workflow implementations, tests, and
documentation define:

- the JS/TS npm package profile's npm package tarball as the initial producer artifact for GitHub
  Release asset publisher composition;
- JS/TS npm package profile outputs or internal handoff artifacts that provide the generic
  producer-to-publisher fields required by ADR 0050;
- the GitHub Release asset publisher contract in producer-neutral terms rather than npm-specific
  terms;
- publisher verification of npm tarball bytes, expected digest, producer provenance signature,
  trusted producer `builder.id`, trusted producer `buildType`, producer subject name, producer
  subject digest, source identity, and release asset target before upload;
- redistribution of the unchanged JS/TS npm package producer provenance bundle as the GitHub Release
  sidecar selected by ADR 0051;
- rejection of raw npm tarballs or arbitrary files that lack acceptable producer provenance;
- documentation that future producer profiles can compose with the publisher by satisfying the same
  handoff contract;
- tests proving that the publisher does not depend on npm-only fields except through producer policy
  checks for this specific composition.

Implementation review should verify that the GitHub Release asset publisher cannot silently become
an npm-specific publisher, cannot accept unauthenticated tarball transport as proof of artifact
trust, and cannot upload a release asset whose bytes, final asset name, or digest differ from the
verified producer provenance subject.

## Pros and Cons of the Options

### Use the JS/TS npm package tarball as the initial producer composition

The JS/TS npm package profile produces the package tarball and SLSA provenance. The publisher
verifies the handoff, uploads the tarball to an existing GitHub Release, and publishes the unchanged
producer provenance sidecar.

- Good, because it composes existing profile decisions into the first concrete release asset path.
- Good, because it keeps npm ecosystem semantics inside the npm producer profile.
- Good, because the publisher contract can be tested with a real producer before additional
  producers exist.
- Neutral, because the first supported producer composition is npm-package-specific.
- Bad, because it does not help projects whose primary release assets are binaries, archives, or
  containers.

### Create a generic release asset builder profile first

The project would define a builder that accepts arbitrary files, generates new provenance, and then
publishes those files as GitHub Release assets.

- Good, because it appears flexible for many asset types.
- Bad, because it reverses ADR 0049's decision to separate artifact production from publication.
- Bad, because generic arbitrary-file provenance would have weak or unclear source-to-artifact
  semantics.
- Bad, because it risks accepting raw caller-produced artifacts under a production SLSA-equivalent
  identity.

### Wait for a Go binary or archive producer profile

The project would defer release asset publisher composition until a Go binary or archive producer
profile exists.

- Good, because Go archives are likely to be common GitHub Release assets for this project family.
- Good, because it avoids npm-specific first examples.
- Bad, because it delays validation of the publisher handoff even though a specified npm producer is
  already available.
- Bad, because it leaves npm package tarball redistribution unspecified despite being a natural
  first composition.

### Leave initial producer composition unspecified

The project would write only an abstract publisher contract and wait for future producers to bind to
it.

- Good, because it maximizes initial abstraction.
- Bad, because specifications and tests would lack a concrete end-to-end composition.
- Bad, because untested abstraction may hide fields that are too generic, too npm-specific, or
  insufficient for real producer handoff.

## More Information

This decision follows ADR 0049, ADR 0050, and ADR 0051. It uses the JS/TS npm package profile
defined by ADR 0013 through ADR 0037 as the first producer profile that can hand off artifact bytes
and producer-generated SLSA provenance to the GitHub Release asset publisher.

This decision does not add a generic file producer, Go binary/archive producer, container producer,
SBOM producer, checksum producer, publisher-owned publication predicate, raw artifact upload mode,
or lower-assurance mirror workflow. Those remain future producer-profile or workflow decisions.

Reference points considered:

- ADR 0049 requires ecosystem-specific producer profiles to own source-to-artifact provenance.
- ADR 0050 requires the publisher to verify producer provenance before release mutation.
- ADR 0051 requires unchanged producer provenance redistribution as a release asset sidecar.
- The JS/TS npm package profile already models a package tarball as the physical release artifact
  for npm publication and exposes tarball digest outputs useful for composition.
