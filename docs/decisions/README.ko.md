# 아키텍처 결정 기록

<div align="center">

[English](README.md) | 한국어

</div>

`slsa-builder`의 아키텍처 결정 기록(ADR) 모음입니다. 이 디렉터리는 시스템의 **왜(why)**를 담습니다.
즉, 채택한 결정, 기각한 대안, 트레이드오프, 결과를 기록합니다. 이 결정과 선택들에 따른 정확한 관찰
가능 동작은 [`docs/architecture/`](../architecture/)에 명세합니다.

> [!Note]  
> 이 프로젝트는 명세 주도 개발(Spec-Driven Development, SDD)을 따릅니다. 먼저 ADR로 결정하고,
> 아키텍처 문서에서 정확한 동작을 명세한 뒤, 그 명세에 따라 구현합니다.

## SDD 모델

| 계층 | 위치                 | 답하는 질문                  | 포함할 수 있는 것                  |
| ---- | -------------------- | ---------------------------- | ---------------------------------- |
| 결정 | `docs/decisions/`    | 왜 이것을 선택했는가?        | 근거, 트레이드오프, 결과           |
| 명세 | `docs/architecture/` | 시스템이 무엇을 해야 하는가? | 계약, 스키마, 불변식, 예시, 픽스처 |
| 구현 | 소스 트리            | 어떻게 동작하는가?           | 코드, 워크플로우, 테스트, 픽스처   |

구현은 앞서 채택한 ADR과 모순되어서는 안 됩니다. 명세 초안 작성 중 기존에 채택한 ADR과 모순이
발견되거나 설계 변경이 필요할 경우, ADR 본문을 수정하지 말고 새 ADR을 작성하세요.

## ADR 목록

ADR 파일은 MADR 4.0.0 문서이며, 네 자리 순번과 kebab-case 제목을 사용합니다. 현재 순번은 `0000`부터
`0054`까지입니다.

| 범위      | 주제                         | 설명                                                       |
| --------- | ---------------------------- | ---------------------------------------------------------- |
| 0000–0012 | 저장소 기반 및 개발 도구     | ADR 형식, 저장소 시작, Go, 린팅, 포맷팅, 도구 체인.        |
| 0013–0037 | JS/TS npm 패키지 profile     | 패키지 선택, build/pack, OIDC publishing, provenance.      |
| 0038–0052 | GitHub Release asset profile | 릴리즈 에셋 subject, publisher 모델, sidecar distribution. |
| 0053–0054 | 릴리즈 매니페스트 메타데이터 | 서명 경계와 release manifest predicate URI.                |

## ADR 추적성

모든 ADR은 명세에 대응되거나, 개발 도구 전용 혹은 대체됨(superseded), 폐기됨(deprecated) 등으로
분류됩니다. 대체되거나 폐기된 ADR은 과거 맥락일 뿐 새로운 명세나 구현을 주도해서는 안 됩니다.

### 채택된 ADR

| ADR  | 결정                                                               | 명세 매핑                                                                                    |
| ---- | ------------------------------------------------------------------ | -------------------------------------------------------------------------------------------- |
| 0000 | Use Markdown Architectural Decision Records                        | 프로세스; 런타임 명세 불필요                                                                 |
| 0001 | Start slsa-builder as a clean repository                           | 기반 결정; 런타임 명세 불필요                                                                |
| 0002 | Extensible trusted reusable workflow foundation                    | Core profile contract, SLSA provenance, verification policy                                  |
| 0003 | Thin core with profile-owned reusable workflows                    | Core profile contract, SLSA provenance, verification policy                                  |
| 0004 | Go as primary implementation language                              | Core profile contract                                                                        |
| 0005 | Dedicated linter toolchain                                         | 도구 전용                                                                                    |
| 0006 | golangci-lint as Go linter runner                                  | 도구 전용                                                                                    |
| 0007 | ShellCheck for shell glue                                          | 도구 전용                                                                                    |
| 0008 | Dedicated formatters                                               | 도구 전용                                                                                    |
| 0009 | Node.js as development tool runtime                                | 도구 전용; core contract가 신뢰 로직 경계를 설명                                             |
| 0010 | pnpm for Node.js development tooling                               | 도구 전용                                                                                    |
| 0011 | Lefthook for local git hook orchestration                          | 도구 전용                                                                                    |
| 0012 | mise as unified development-tool runtime                           | 도구 전용                                                                                    |
| 0013 | Scope initial JS/TS profile to npm packages                        | JS/TS npm profile, composition spec                                                          |
| 0014 | Support npm, pnpm, and Yarn for initial build stages               | JS/TS npm build and pack                                                                     |
| 0015 | Manifest-first package manager selection                           | JS/TS npm build and pack                                                                     |
| 0016 | Corepack for pnpm and Yarn build stages                            | JS/TS npm build and pack                                                                     |
| 0017 | Explicit package manager version enforcement                       | JS/TS npm build and pack                                                                     |
| 0018 | Publish one JS/TS package per profile run                          | JS/TS npm profile, build and pack                                                            |
| 0019 | Validate package metadata through packed artifacts                 | JS/TS npm build and pack                                                                     |
| 0022 | `js-ts-npm-package-slsa3.yml` workflow entrypoint                  | JS/TS npm profile                                                                            |
| 0023 | `package-directory` as required package selector                   | JS/TS npm profile, build and pack                                                            |
| 0024 | OIDC trusted publishing without publish secrets                    | JS/TS npm profile, provenance and publish                                                    |
| 0025 | Return package identity and tarball digest outputs                 | JS/TS npm provenance and publish                                                             |
| 0026 | Document supported release caller patterns and runtime guards      | JS/TS npm profile                                                                            |
| 0027 | GitHub-Hosted Ubuntu 24.04 and Node.js 24 runtime                  | JS/TS npm profile, build and pack                                                            |
| 0028 | SHA-pinned reusable workflow builder identity                      | Identity and build types, common provenance, release manifest, verification policy           |
| 0029 | Windlass-generated SLSA provenance for npm publish                 | Common provenance, JS/TS npm provenance and publish, verification policy                     |
| 0030 | Accept registry URL while guaranteeing only npmjs semantics        | JS/TS npm profile, provenance and publish, verification policy                               |
| 0031 | Sigstore-signed in-toto release manifest                           | Identity and build types, release manifest                                                   |
| 0032 | Constrain manual dispatch releases to version tags                 | JS/TS npm profile                                                                            |
| 0033 | Run build script only when declared                                | JS/TS npm build and pack                                                                     |
| 0034 | Do not support private dependency credentials                      | JS/TS npm profile                                                                            |
| 0035 | `actions/attest` as initial Sigstore signing adapter               | Core profile contract, common provenance, JS/TS npm provenance and publish, release manifest |
| 0036 | Three-job digest-verified publish graph                            | JS/TS npm provenance and publish, verification policy                                        |
| 0037 | Define initial verification deliverables                           | Verification policy and fixtures                                                             |
| 0039 | Scope release asset profile to one asset per run                   | GitHub Release asset publisher                                                               |
| 0042 | Use acquired domains for buildType URIs                            | Core profile contract, identity and build types                                              |
| 0043 | Upload release assets to existing releases                         | GitHub Release asset publisher                                                               |
| 0045 | Use release asset name as SLSA subject name                        | GitHub Release asset publisher                                                               |
| 0046 | Keep checksums and SBOMs out of subject digest                     | GitHub Release asset publisher                                                               |
| 0048 | Make linked artifacts storage records explicit opt-in              | GitHub Release asset publisher                                                               |
| 0049 | Separate artifact production from GitHub Release asset publication | Identity and build types, GitHub Release asset publisher, verification policy                |
| 0050 | Define producer-to-publisher handoff contract                      | GitHub Release asset publisher, verification policy                                          |
| 0051 | Distribute producer provenance with release assets                 | GitHub Release asset publisher, verification policy                                          |
| 0052 | Compose npm package tarball producer with release asset publisher  | Composition spec, verification policy                                                        |
| 0053 | Three-job release manifest signing boundary                        | Release manifest                                                                             |
| 0054 | Use `slsa-builder.dev` release manifest predicate URI              | Release manifest, verification policy                                                        |

### 대체 혹은 폐기된 ADR (과거 맥락으로만 참조)

| ADR  | 다음으로 대체 | 이유                                                                 |
| ---- | ------------- | -------------------------------------------------------------------- |
| 0020 | 0028          | reusable workflow ref identity를 SHA-pinned identity로 대체함        |
| 0021 | 0042          | profile-specific buildType URI를 별도로 확보한 도메인 URI로 대체함   |
| 0038 | 0049          | release asset 출처 모델을 builder에서 distributor로 변경함           |
| 0040 | 0049          | `github-release-asset-slsa3.yml` 진입점을 production path에서 제거함 |
| 0041 | 0042          | release asset buildType URI를 별도의 도메인 네임스페이스로 대체함    |
| 0044 | 0049          | 3단계 release asset build 그래프를 publisher 모델로 대체함           |
| 0047 | 0051          | canonical attestation storage를 sidecar distribution 모델로 대체함   |

## ADR 작성 규칙

1. **ADR 하나당 결정 하나.** 나중에 대체하거나 참조하기 쉽도록 결정 범위를 좁게 유지합니다.
2. **다음 순번 사용.** `0000-title.md` 형식의 연속된 번호 체계를 이어갑니다.
3. **MADR 4.0.0 사용.** 해당 결정에 명백히 필요 없는 경우가 아니라면 기존 섹션 구조를 유지합니다.
4. **Human Era 날짜 사용.** ADR front matter 날짜는 `12026-07-08` 형식을 사용합니다.
5. **채택된 ADR 본문 불변.** 채택 후에는 `status` 필드만 업데이트하고, 결정이 바뀌면 새 ADR을
   작성합니다.
6. **추적 표 업데이트.** 새 ADR을 위 목록에 추가하고, 관찰 가능 동작을 정의한다면 아키텍처 명세
   링크도 업데이트합니다.

## 새 ADR 추가 방법

1. 기존 ADR 또는 `0000-use-markdown-architectural-decision-records.md`의 구조를 복사합니다.
2. 다음 네 자리 순번과 kebab-case 제목을 부여합니다.
3. 결정의 맥락, 선택지, 결과, 영향, 확인 기준을 기록합니다.
4. 위 추적성 표에 ADR을 추가합니다.
5. 결정이 관찰 가능 동작을 바꾼다면 [`docs/architecture/`](../architecture/)의 관련 명세를
   업데이트합니다.
6. 제출 전 문서 품질 검사 명령을 실행합니다.

## ADR PR 검증 체크리스트

- [ ] ADR 번호가 순차적이고 제목이 kebab-case입니다.
- [ ] ADR이 MADR 4.0.0 구조와 Human Era 날짜 형식을 사용합니다.
- [ ] 채택된 ADR 본문을 소급 수정하지 않았습니다.
- [ ] 추적성 표가 ADR을 명세, 도구 전용, 또는 과거 상태로 매핑합니다.
- [ ] ADR이 관찰 가능 동작을 바꾸는 경우 아키텍처 명세를 업데이트했습니다.
- [ ] 린팅 및 포맷팅을 통과합니다.

  ```bash
  pnpm exec prettier --check "docs/**/*.md"
  pnpm exec markdownlint-cli2 "docs/**/*.md"
  ```
