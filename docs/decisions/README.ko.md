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
`0112`까지입니다.

| ADR                  | 주제                                        | 설명                                                                                                                                                                                                                   |
| -------------------- | ------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0000–0012            | 저장소 기반 및 초기 개발 도구               | ADR 형식, 클린 저장소 시작, 신뢰 워크플로우 기반, Go, 린팅, 포맷팅, Node.js, pnpm, Lefthook, mise.                                                                                                                     |
| 0013–0037            | JS/TS npm 패키지 프로필                     | 패키지 선택, 매니페스트 우선 매니저 선택, build/pack, 워크플로우 진입점, OIDC 게시, provenance, 초기 서명 어댑터와 과거 결정.                                                                                          |
| 0038–0052            | GitHub Release 에셋 프로필                  | 릴리스 에셋 subject, buildType URI, 퍼블리셔 모델, 생산자 인계, sidecar 배포, npm composition과 과거 결정.                                                                                                             |
| 0053–0054, 0062      | 릴리스 매니페스트 및 신뢰 생산자 정책       | 3개 작업 서명 경계, 릴리스 매니페스트 predicate URI, 신뢰 생산자 정책 교집합과 충돌 처리.                                                                                                                              |
| 0055                 | 서명 어댑터 Statement 구성                  | `actions/attest` custom mode와 서명 후 Statement 검사. 이후 Go-native 서명기로 대체.                                                                                                                                   |
| 0056                 | 비선택 lockfile 진단                        | 매니페스트가 선택한 패키지 매니저 환경의 stale lockfile 처리.                                                                                                                                                          |
| 0057–0060, 0108      | 공개 npm 릴리스 인터페이스 및 퍼블리셔 권한 | 단일 공개 진입점, 최소 `workflow_call` 인터페이스, 동일 저장소 대상과 최소 권한 퍼블리셔, 분리된 내부 권한, 제거된 `release-tag` 및 `provenance-sidecar` 입력, built tag 파생과 필수 sidecar 배포.                     |
| 0061                 | 서명된 Statement JSON 파싱                  | 서명된 SLSA Statement의 의미 검증 전 중복 JSON member 거부.                                                                                                                                                            |
| 0063, 0090           | 소비자 pnpm/Yarn 지원 경계                  | Yarn Berry 지원 메타데이터와 Corepack 이력, 소비자 pnpm 범위 `[10.0.0, 12.0.0)` 및 지원 표에 연동된 경계.                                                                                                              |
| 0064                 | npm provenance subject 호환성               | 동일 tarball 바이트에 대한 SHA-512와 SHA-256을 사용하는 npm PURL subject.                                                                                                                                              |
| 0065                 | ADR 생애주기 메타데이터                     | 닫힌 status 문법, 분리된 relations 필드, 양방향 추적성.                                                                                                                                                                |
| 0066–0067, 0074–0076 | 공통 실행 수렴 및 변경 정책                 | Job-class concurrency, run-identity 멱등성, 단일 job mutation segment, 탐지 기반 cross-run 안전, `queue: max` FIFO, 관측 preflight와 첫 mutation 분류.                                                                 |
| 0068–0069            | 검증 신원 및 투명성 신뢰                    | 불변 builder 및 source 신원, Rekor inclusion, 오프라인 검증, Sigstore trust root 거버넌스.                                                                                                                             |
| 0070–0071            | 빌드 환경 및 빌더 기록                      | 패키지 매니저 배포본, 초기 runner image descriptor, `builder.version` 및 `builderDependencies`.                                                                                                                        |
| 0072                 | 릴리스 에셋 실행 소유권 바인딩              | Sidecar-first pair 바인딩과 릴리스 에셋 custody 비귀속.                                                                                                                                                                |
| 0073, 0102           | npm 게시 수렴 및 읽기 확인 분류             | Published-attestation run identity, E422의 축소된 read-back budget, determinate absence, 버전이 있으면 full budget 유지.                                                                                               |
| 0077                 | Windlass provenance 서명 어댑터             | 모든 프로필의 정확한 바이트를 서명하는 Go-native Sigstore DSSE. `0035`와 `0055`를 대체.                                                                                                                                |
| 0078                 | pnpm 설정 전용 루트 패키지 모드             | `pnpm-workspace.yaml`의 `packages` 누락은 잘못된 workspace 데이터가 아니라 루트 전용 모드로 선택.                                                                                                                      |
| 0079–0080            | 호출 소스 재시도 및 호출 바인딩             | 태그 전용 `source-ref`, 고정 파이프라인 릴리스 재시도, 서명된 필드에 바인딩된 소스 정책과 호출 컨텍스트로서의 인증서 클레임.                                                                                           |
| 0081                 | npm OIDC 교환 응답 계약                     | 실측 성공 응답 형태와 15분 exchange 토큰 수명.                                                                                                                                                                         |
| 0082–0084            | 게시 단계 npm CLI 프로비저닝                | 검토된 allowlist, npm/cli#9882를 포함하는 초기 핀, 다이제스트 검증 npm registry tarball과 커밋된 SHA-512.                                                                                                              |
| 0085, 0098–0099      | 소비자 빌드 단계 Node.js 런타임             | 정확한 Node.js 24 패치와 번들 npm 쌍, SHA-256으로 검증한 release tarball, `resolvedDependencies`에 런타임 기록.                                                                                                        |
| 0086–0087, 0089      | 저장소 개발 Node.js 및 pnpm 도구            | 저장소 전용 mise 필드 해석, `package.json` 핀, `devEngines.runtime`, `devEngines.packageManager`, pnpm 12 개발 도구.                                                                                                   |
| 0088, 0091           | 소비자 패키지 매니저 프로비저닝 및 선택     | 빌드 단계 pnpm/Yarn registry tarball로 Corepack 대체, rollout 경계, `devEngines.packageManager`를 통한 소비자 패키지 매니저 일관 선택.                                                                                 |
| 0092–0094            | 디스크립터 무결성 다이제스트 문법           | 선택적 선언 다이제스트의 fail-closed 검증, W3C SRI 알고리즘 토큰, `+algorithm.<hex>` 접미사, SRI base64 거부, 기록 모델 불변.                                                                                          |
| 0095                 | npm 디스크립터 의미                         | 선언 버전 불일치 경고, npm 다이제스트 거부, 별도의 npm CLI 자체 강제 경계.                                                                                                                                             |
| 0096–0097            | 프로젝트 및 패킹된 `.corepack.env` 정책     | `COREPACK_ENV_FILE=0`, 프로젝트 파일 존재 시 비치명적 경고, 파서 없는 post-pack 거부.                                                                                                                                  |
| 0100, 0111–0112      | 러너 이미지 증거 및 보고 신원               | 온디스크 이미지 메타데이터, Included Software URI, 정확한 full `image_version`, 비권위 `deployment_version`, 디코딩된 `Operating System.detail`, 진단 전용 `$ImageOS`, 오프라인 projection.                            |
| 0101                 | 외부 CLI stderr 위생 릴레이                 | Allowlisted 서브프로세스 stderr, 명세 소유 위생 필터, enrichment로 분류, canonical 계약 봉인 유지.                                                                                                                     |
| 0103–0107, 0109      | 호출 신원 및 외부 파라미터                  | 공통 workflow 호출 신원, builder 삼중 필드 제거, `invocation-source` 및 `built-source`, 외부 소스 통합, `internalParameters.github`의 `event_name`, package-manifest descriptor, 실제 interface 입력과 recompute 규칙. |
| 0110                 | Non-runner dependency 미디어 타입           | 구체적 artifact에 deterministic `mediaType` 부여, 타입 없는 source descriptor, `downloadLocation` 부재.                                                                                                                |

## ADR status와 relations

ADR 0065는 이 디렉터리 모든 ADR의 생애주기·추적성 메타데이터 문법을 정의합니다.

### Status 문법

`status` frontmatter 필드는 다음 형태 중 정확히 하나만 사용해야 합니다.

```text
proposed
rejected
accepted
deprecated
superseded by ADR-XXXX
```

`accepted; partially updated by ...`나 `amended by ...`처럼 합성하거나 서술형으로 쓴 status 값은
유효하지 않습니다. `deprecated`는 해당 결정이 더 이상 권장되지 않으며 대체 ADR이 없거나 필요하지
않은 경우입니다. `superseded by ADR-XXXX`는 지정한 ADR이 그 결정을 전체적으로 대체하는 경우입니다.

### Relations 필드

ADR 간 관계는 `status`가 아니라 frontmatter `relations` 필드에 기록합니다.

```yaml
relations:
  - type: partially-superseded-by
    target: ADR-0049
    scope: "Release asset profile의 buildType URI 배정 및 관련 confirmation 조항"
```

각 항목은 `type`, 대상 ADR 번호인 `target`, 그리고 `scope`(부분 대체·개정 관계에서는 필수이며,
영향받는 조항을 특정)를 가집니다. 관계 어휘는 닫혀 있습니다.

| 정방향 (신 ADR)        | 역방향 (구 ADR)           | 의미                                    | 구 ADR status 변화         |
| ---------------------- | ------------------------- | --------------------------------------- | -------------------------- |
| `supersedes`           | `superseded-by`           | 신 ADR이 구 ADR을 전체 대체             | `superseded by ...`로 변경 |
| `partially-supersedes` | `partially-superseded-by` | 명시한 조항만 대체, 나머지는 유효       | `accepted` 유지            |
| `amends`               | `amended-by`              | 세부 사항을 축소·조정하거나 예외를 추가 | `accepted` 유지            |
| `see-also`             | `see-also`                | 정보성 상호 참조                        | 없음                       |

구현자가 구 조항을 문자 그대로 더 이상 따를 수 없으면 `partially-supersedes`, 구 조항이 여전히
지배적이며 신 ADR이 그것을 한정하기만 하면 `amends`를 사용합니다. 모호한 경우는
`partially-supersedes`로 해결합니다.

관계선은 양방향입니다. 신 ADR이 대상에 대해 `supersedes`, `partially-supersedes`, `amends`를
선언하면, 같은 변경에서 대상 ADR의 `relations` 필드에 대응하는 역방향 항목을 추가해야 합니다. 신
ADR은 영향을 주는 모든 선행 ADR을 열거해야 하며, 누락은 추적성 결함입니다. 결함은 빠진 역방향 항목을
추가해 복구하며, 복구만을 위한 새 ADR은 필요 없고 본문은 절대 편집하지 않습니다.

채택 후에는 `status`와 `relations` frontmatter 필드만 변경할 수 있습니다. 전체 결정 내용은
[ADR 0065](0065-use-closed-status-grammar-with-separate-relations-field.md)를 참조하세요.

## ADR 추적성

모든 ADR은 명세에 대응되거나, 개발 도구 전용 혹은 대체됨(superseded), 폐기됨(deprecated) 등으로
분류됩니다. 대체되거나 폐기된 ADR은 과거 맥락일 뿐 새로운 명세나 구현을 주도해서는 안 됩니다. 부분
대체·개정 관계(ADR 0065)는 ADR을 이 표에서 이동시키지 않으며, 각 ADR의 `relations` frontmatter
필드에 기록합니다.

### 채택된 ADR

| ADR  | 결정                                                                                                              | 명세 매핑                                                                                                                                                                           |
| ---- | ----------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0000 | Use Markdown Architectural Decision Records                                                                       | 프로세스. 런타임 명세 불필요                                                                                                                                                        |
| 0001 | Start slsa-builder as a clean repository                                                                          | 기반 결정. 런타임 명세 불필요                                                                                                                                                       |
| 0002 | Extensible trusted reusable workflow foundation                                                                   | Core profile contract, SLSA provenance, verification policy                                                                                                                         |
| 0003 | Thin core with profile-owned reusable workflows                                                                   | Core profile contract, SLSA provenance, verification policy                                                                                                                         |
| 0004 | Go as primary implementation language                                                                             | Core profile contract                                                                                                                                                               |
| 0005 | Dedicated linter toolchain                                                                                        | 도구 전용                                                                                                                                                                           |
| 0006 | golangci-lint as Go linter runner                                                                                 | 도구 전용                                                                                                                                                                           |
| 0007 | ShellCheck for shell glue                                                                                         | 도구 전용                                                                                                                                                                           |
| 0008 | Dedicated formatters                                                                                              | 도구 전용                                                                                                                                                                           |
| 0009 | Node.js as development tool runtime                                                                               | 도구 전용. core contract가 신뢰 로직 경계를 설명                                                                                                                                    |
| 0010 | pnpm for Node.js development tooling                                                                              | 도구 전용                                                                                                                                                                           |
| 0011 | Lefthook for local git hook orchestration                                                                         | 도구 전용                                                                                                                                                                           |
| 0012 | mise as unified development-tool runtime                                                                          | 도구 전용                                                                                                                                                                           |
| 0013 | Scope initial JS/TS profile to npm packages                                                                       | JS/TS npm profile, composition spec                                                                                                                                                 |
| 0014 | Support npm, pnpm, and Yarn for initial build stages                                                              | JS/TS npm build and pack                                                                                                                                                            |
| 0015 | Manifest-first package manager selection                                                                          | JS/TS npm build and pack                                                                                                                                                            |
| 0016 | Corepack for pnpm and Yarn build stages                                                                           | JS/TS npm build and pack                                                                                                                                                            |
| 0017 | Explicit package manager version enforcement                                                                      | JS/TS npm build and pack                                                                                                                                                            |
| 0018 | Publish one JS/TS package per profile run                                                                         | JS/TS npm profile, build and pack                                                                                                                                                   |
| 0019 | Validate package metadata through packed artifacts                                                                | JS/TS npm build and pack                                                                                                                                                            |
| 0022 | `js-ts-npm-package-slsa3.yml` workflow entrypoint                                                                 | JS/TS npm profile                                                                                                                                                                   |
| 0023 | `package-directory` as required package selector                                                                  | JS/TS npm profile, build and pack                                                                                                                                                   |
| 0024 | OIDC trusted publishing without publish secrets                                                                   | JS/TS npm profile, provenance and publish                                                                                                                                           |
| 0025 | Return package identity and tarball digest outputs                                                                | JS/TS npm provenance and publish                                                                                                                                                    |
| 0026 | Document supported release caller patterns and runtime guards                                                     | JS/TS npm profile                                                                                                                                                                   |
| 0027 | GitHub-Hosted Ubuntu 24.04 and Node.js 24 runtime                                                                 | JS/TS npm profile, build and pack                                                                                                                                                   |
| 0028 | SHA-pinned reusable workflow builder identity                                                                     | Identity and build types, common provenance, release manifest, verification policy                                                                                                  |
| 0029 | Windlass-generated SLSA provenance for npm publish                                                                | Common provenance, JS/TS npm provenance and publish, verification policy                                                                                                            |
| 0030 | Accept registry URL while guaranteeing only npmjs semantics                                                       | JS/TS npm profile, provenance and publish, verification policy                                                                                                                      |
| 0031 | Sigstore-signed in-toto release manifest                                                                          | Identity and build types, release manifest                                                                                                                                          |
| 0032 | Constrain manual dispatch releases to version tags                                                                | JS/TS npm profile                                                                                                                                                                   |
| 0033 | Run build script only when declared                                                                               | JS/TS npm build and pack                                                                                                                                                            |
| 0034 | Do not support private dependency credentials                                                                     | JS/TS npm profile                                                                                                                                                                   |
| 0035 | `actions/attest` as initial Sigstore signing adapter                                                              | Core profile contract, common provenance, JS/TS npm provenance and publish, release manifest                                                                                        |
| 0036 | Three-job digest-verified publish graph                                                                           | JS/TS npm provenance and publish, verification policy                                                                                                                               |
| 0037 | Define initial verification deliverables                                                                          | Verification policy and fixtures                                                                                                                                                    |
| 0039 | Scope release asset profile to one asset per run                                                                  | GitHub Release asset publisher                                                                                                                                                      |
| 0042 | Use acquired domains for buildType URIs                                                                           | Core profile contract, identity and build types                                                                                                                                     |
| 0043 | Upload release assets to existing releases                                                                        | GitHub Release asset publisher                                                                                                                                                      |
| 0045 | Use release asset name as SLSA subject name                                                                       | GitHub Release asset publisher                                                                                                                                                      |
| 0046 | Keep checksums and SBOMs out of subject digest                                                                    | GitHub Release asset publisher                                                                                                                                                      |
| 0048 | Make linked artifacts storage records explicit opt-in                                                             | GitHub Release asset publisher                                                                                                                                                      |
| 0049 | Separate artifact production from GitHub Release asset publication                                                | Identity and build types, GitHub Release asset publisher, verification policy                                                                                                       |
| 0050 | Define producer-to-publisher handoff contract                                                                     | GitHub Release asset publisher, verification policy                                                                                                                                 |
| 0051 | Distribute producer provenance with release assets                                                                | GitHub Release asset publisher, verification policy                                                                                                                                 |
| 0052 | Compose npm package tarball producer with release asset publisher                                                 | Composition spec, verification policy                                                                                                                                               |
| 0053 | Three-job release manifest signing boundary                                                                       | Release manifest                                                                                                                                                                    |
| 0054 | Use `slsa-builder.dev` release manifest predicate URI                                                             | Release manifest, verification policy                                                                                                                                               |
| 0055 | `actions/attest` custom mode for Statement construction                                                           | Common provenance, JS/TS npm provenance and publish                                                                                                                                 |
| 0056 | Treat non-selected lockfiles as stale diagnostics                                                                 | JS/TS npm build and pack, JS/TS npm provenance and publish, verification policy                                                                                                     |
| 0057 | Provide public npm release-asset mode                                                                             | Composition spec, JS/TS npm provenance and publish, GitHub Release asset publisher                                                                                                  |
| 0058 | Define GitHub Release asset publisher authority boundary                                                          | GitHub Release asset publisher, composition spec, verification policy                                                                                                               |
| 0059 | Define public npm release-asset mode interface                                                                    | Composition spec, JS/TS npm provenance and publish, GitHub Release asset publisher                                                                                                  |
| 0060 | Unify npm profile public entrypoint with release-asset mode                                                       | JS/TS npm profile, composition spec, verification policy                                                                                                                            |
| 0061 | Reject duplicate JSON members in signed SLSA Statements                                                           | Common provenance, JS/TS npm provenance and publish, verification policy                                                                                                            |
| 0062 | Intersect trusted producer policies                                                                               | Release manifest, GitHub Release asset publisher, verification policy                                                                                                               |
| 0063 | Limit Yarn support to Berry v4 with Corepack metadata                                                             | JS/TS npm build and pack                                                                                                                                                            |
| 0064 | Use npm PURL subject with SHA-512 and SHA-256 digests                                                             | Common provenance, JS/TS npm specs, composition and publisher specs, verification policy                                                                                            |
| 0065 | Use a closed status grammar and a separate relations field                                                        | 프로세스. 런타임 명세 불필요                                                                                                                                                        |
| 0066 | Serialize release mutations with job-class concurrency                                                            | GitHub Release asset publisher, JS/TS npm provenance and publish, release manifest, JS/TS npm package profile, verification policy                                                  |
| 0067 | Converge repeated runs within run identity                                                                        | Release manifest, GitHub Release asset publisher, JS/TS npm provenance and publish, verification policy                                                                             |
| 0068 | Bind verification to immutable builder and source identities                                                      | Verification policy, identity and build types, release manifest, JS/TS npm provenance and publish                                                                                   |
| 0069 | Require Rekor transparency and govern the Sigstore trust root                                                     | Verification policy, common provenance, release manifest                                                                                                                            |
| 0070 | Record package manager distributions and runner image in resolvedDependencies                                     | SLSA provenance v1, JS/TS npm provenance and publish, JS/TS npm build and pack, verification policy and fixtures                                                                    |
| 0071 | Activate builder.version and builderDependencies for platform components                                          | SLSA provenance v1, JS/TS npm provenance and publish, verification policy and fixtures                                                                                              |
| 0072 | Use sidecar-first pair binding for release asset run ownership                                                    | GitHub Release asset publisher, npm-to-release-asset composition, verification policy and fixtures                                                                                  |
| 0073 | Require published-attestation run identity for npm same-run convergence                                           | JS/TS npm provenance and publish, verification policy and fixtures                                                                                                                  |
| 0074 | Use single-job mutation segments with detection-based cross-run safety                                            | GitHub Release asset publisher, release manifest, npm-to-release-asset composition, verification policy and fixtures                                                                |
| 0075 | Queue mutation segment contenders with queue: max                                                                 | GitHub Release asset publisher, JS/TS npm provenance and publish, release manifest, verification policy and fixtures                                                                |
| 0076 | Use observation preflights and first-mutation classification                                                      | GitHub Release asset publisher, JS/TS npm package profile, verification policy and fixtures                                                                                         |
| 0077 | Use a Go-native Sigstore DSSE signer for Windlass provenance signing                                              | Core profile contract, SLSA provenance v1, JS/TS npm package profile, JS/TS npm provenance and publish, release asset publisher, release manifest, composition, verification policy |
| 0078 | Treat settings-only pnpm-workspace.yaml as standalone root package mode                                           | JS/TS npm build and pack, verification policy and fixtures                                                                                                                          |
| 0079 | Support a tags-only caller-specified build source ref for release retries across profiles                         | Core profile contract, JS/TS npm package profile, JS/TS npm provenance and publish, verification policy and fixtures                                                                |
| 0080 | Bind source identity policy to signed provenance fields, certificate claims as invocation context                 | Core profile contract, SLSA provenance v1, identity and build types, JS/TS npm provenance and publish, verification policy and fixtures                                             |
| 0081 | Pin the npm OIDC exchange response contract to the observed shape, correct token lifetime                         | JS/TS npm package profile, JS/TS npm provenance and publish, verification policy and fixtures                                                                                       |
| 0082 | Pin the publish-stage npm CLI version with integrity-verified provisioning and a reviewed allowlist               | JS/TS npm package profile, JS/TS npm provenance and publish, verification policy and fixtures                                                                                       |
| 0083 | Defer the npm M1 publish remediation to the upstream provenance-file fix                                          | JS/TS npm package profile, JS/TS npm provenance and publish, verification policy and fixtures                                                                                       |
| 0084 | Provision the publish-stage npm CLI from a digest-verified registry tarball                                       | JS/TS npm package profile, JS/TS npm provenance and publish, verification policy and fixtures                                                                                       |
| 0085 | Pin the Node.js 24 patch version and assert the expected bundled npm pair                                         | JS/TS npm package profile, JS/TS npm build and pack, JS/TS npm provenance and publish, verification policy and fixtures                                                             |
| 0086 | Provision pnpm through mise packageManager-field resolution instead of Corepack                                   | 도구 전용                                                                                                                                                                           |
| 0087 | Adopt pnpm 12 for Node.js development tooling                                                                     | 도구 전용                                                                                                                                                                           |
| 0088 | Provision build-stage pnpm and Yarn from digest-verified npm registry tarballs                                    | JS/TS npm build and pack, verification policy and fixtures(현재는 Corepack 지원 윈도우 경계만 명세됐으며 registry tarball 메커니즘은 후속 명세 작업(B01/#106)으로 유보됨)           |
| 0089 | Provision the development Node.js 런타임 through mise package.json field resolution                               | 도구 전용                                                                                                                                                                           |
| 0090 | Extend the supported pnpm range to the 10.x line                                                                  | JS/TS npm build and pack, verification policy and fixtures(Corepack 윈도우 pnpm 지원 하한 개정)                                                                                     |
| 0091 | Accept devEngines.packageManager as a uniform selection source                                                    | JS/TS npm build and pack(package manager selection), verification policy and fixtures                                                                                               |
| 0092 | Accept optional integrity digests in package-manager descriptors                                                  | JS/TS npm build and pack(descriptor grammar, fail-closed digest reconciliation), verification policy and fixtures                                                                   |
| 0093 | Pin descriptor digest algorithms to the SRI set                                                                   | JS/TS npm build and pack(descriptor digest grammar), verification policy and fixtures                                                                                               |
| 0094 | Pin the descriptor digest format to a hex suffix                                                                  | JS/TS npm build and pack(descriptor digest grammar), verification policy and fixtures                                                                                               |
| 0095 | Accept npm version declarations with a mismatch warning and reject npm digests                                    | JS/TS npm build and pack(npm descriptor semantics), verification policy and fixtures, README                                                                                        |
| 0096 | Neutralize the project .corepack.env with a pinned COREPACK_ENV_FILE=0 and warn on its presence                   | JS/TS npm build and pack(credential-configuration preflight), verification policy and fixtures                                                                                      |
| 0097 | Reject a packed .corepack.env at the post-pack artifact gate                                                      | JS/TS npm build and pack(post-pack artifact gate), verification policy and fixtures                                                                                                 |
| 0098 | Provision the build-stage Node.js 런타임 from a digest-verified release tarball                                   | JS/TS npm package profile, JS/TS npm build and pack, JS/TS npm provenance and publish, verification policy and fixtures                                                             |
| 0099 | Record the digest-verified Node.js 런타임 in resolvedDependencies                                                 | SLSA provenance v1, JS/TS npm provenance and publish, JS/TS npm build and pack, verification policy and fixtures                                                                    |
| 0100 | Record runner image identity from on-disk metadata with the control-plane deployment version as separate evidence | JS/TS npm build and pack(runner image capture), verification policy and fixtures                                                                                                    |
| 0101 | Relay external CLI stderr to logs under hygiene bounds, with template classification as enrichment                | Core profile contract(external-process operational diagnostics), JS/TS npm provenance and publish(best-effort operational diagnostics)                                              |
| 0102 | Classify npm publish E422 rejections as determinate pre-commit failures under a reduced read-back budget          | JS/TS npm provenance and publish, verification policy and fixtures                                                                                                                  |
| 0103 | Define external workflow parameters as common invocation identity                                                 | Core profile contract, SLSA provenance v1, identity and build types, JS/TS npm provenance and publish, verification policy and fixtures                                             |
| 0104 | Remove redundant external-parameter builder record                                                                | Core profile contract, SLSA provenance v1, identity and build types, JS/TS npm provenance and publish, verification policy and fixtures                                             |
| 0105 | Record invocation and built source as role-named resolved dependencies                                            | Core profile contract, SLSA provenance v1, JS/TS npm provenance and publish, verification policy and fixtures                                                                       |
| 0106 | Consolidate external source parameters onto workflow and descriptor bindings                                      | Core profile contract, SLSA provenance v1, JS/TS npm provenance and publish, verification policy and fixtures                                                                       |
| 0107 | Record invocation event metadata in internalParameters.github only                                                | Core profile contract, SLSA provenance v1, JS/TS npm provenance and publish, verification policy and fixtures                                                                       |
| 0108 | Remove redundant npm release-asset inputs                                                                         | JS/TS npm package profile, JS/TS npm provenance and publish, npm-to-release-asset composition, GitHub Release asset publisher, verification policy and fixtures                     |
| 0109 | Minimize external parameters to actual interface inputs                                                           | Core profile contract, SLSA provenance v1, JS/TS npm package profile, JS/TS npm build and pack, JS/TS npm provenance and publish, verification policy and fixtures                  |
| 0110 | Record deterministic media types for non-runner dependencies                                                      | SLSA provenance v1, JS/TS npm provenance and publish, JS/TS npm build and pack, verification policy and fixtures                                                                    |
| 0111 | Record runner-image reported identity with the Included Software URI and full image version                       | SLSA provenance v1, JS/TS npm provenance and publish, JS/TS npm build and pack, verification policy and fixtures                                                                    |
| 0112 | Record runner-image operating system from on-disk metadata                                                        | SLSA provenance v1, JS/TS npm provenance and publish, JS/TS npm build and pack, verification policy and fixtures                                                                    |

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
4. **Holocene Era 날짜 사용.** ADR front matter 날짜는 `12026-07-08` 형식을 사용합니다.
5. **채택된 ADR 본문 불변.** 채택 후에는 `status`와 `relations` frontmatter 필드만 업데이트하고,
   결정이 바뀌면 새 ADR을 작성합니다.
6. **닫힌 status 문법 사용.** `status`는 `proposed`, `rejected`, `accepted`, `deprecated`,
   `superseded by ADR-XXXX` 중 정확히 하나여야 합니다(ADR 0065).
7. **관계는 양방향으로 선언.** 새 ADR이 선행 ADR을 대체·부분 대체·개정한다면, 자신의 `relations`
   필드에 정방향 항목을 선언하고 각 대상에 역방향 항목을 추가하며, 부분 대체·개정 관계에는 `scope`를
   작성합니다. 영향받는 모든 선행 ADR을 열거하세요. 누락은 추적성 결함입니다.
8. **추적 표 업데이트.** 새 ADR을 위 목록에 추가하고, 관찰 가능 동작을 정의한다면 아키텍처 명세
   링크도 업데이트합니다.

## 새 ADR 추가 방법

1. 기존 ADR 또는 `0000-use-markdown-architectural-decision-records.md`의 구조를 복사합니다.
2. 다음 네 자리 순번과 kebab-case 제목을 부여합니다.
3. 결정의 맥락, 선택지, 결과, 영향, 확인 기준을 기록합니다.
4. 닫힌 문법으로 `status`를 채우고, 대상 ADR의 역방향 엣지를 포함해 필요한 `relations` 항목을
   선언합니다.
5. 위 추적성 표에 ADR을 추가합니다.
6. 결정이 관찰 가능 동작을 바꾼다면 [`docs/architecture/`](../architecture/)의 관련 명세를
   업데이트합니다.
7. 제출 전 문서 품질 검사 명령을 실행합니다.

## ADR PR 검증 체크리스트

- [ ] ADR 번호가 순차적이고 제목이 kebab-case입니다.
- [ ] ADR이 MADR 4.0.0 구조와 Holocene Era 날짜 형식을 사용합니다.
- [ ] 채택된 ADR 본문을 소급 수정하지 않았습니다. `status`와 `relations`만 변경했습니다.
- [ ] `status`가 닫힌 문법을 사용합니다(합성·서술형 값 없음).
- [ ] 관계 type, target, 필수 `scope` 값이 ADR 0065를 따르고, 모든 정방향 관계에 대상 ADR의 역방향
      엣지가 있습니다.
- [ ] 추적성 표가 ADR을 명세, 도구 전용, 또는 과거 상태로 매핑합니다.
- [ ] ADR이 관찰 가능 동작을 바꾸는 경우 아키텍처 명세를 업데이트했습니다.
- [ ] 린팅 및 포맷팅을 통과합니다.

  ```bash
  pnpm exec prettier --check "docs/**/*.md"
  pnpm exec markdownlint-cli2 "docs/**/*.md"
  ```
