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
`0082`까지입니다.

| 범위      | 주제                               | 설명                                                                           |
| --------- | ---------------------------------- | ------------------------------------------------------------------------------ |
| 0000–0012 | 저장소 기반 및 개발 도구           | ADR 형식, 저장소 시작, Go, 린팅, 포맷팅, 도구 체인.                            |
| 0013–0037 | JS/TS npm 패키지 profile           | 패키지 선택, build/pack, OIDC publishing, provenance.                          |
| 0038–0052 | GitHub Release asset profile       | 릴리즈 에셋 subject, publisher 모델, sidecar distribution.                     |
| 0053–0054 | 릴리즈 매니페스트 메타데이터       | 서명 경계와 release manifest predicate URI.                                    |
| 0055      | 서명 어댑터 Statement 구성         | `actions/attest` custom mode와 서명 후 Statement 검사.                         |
| 0056      | 비선택 lockfile 진단               | Manifest-selected manager에서 stale lockfile 처리.                             |
| 0057      | 공개 npm release-asset 모드        | 단일 npm 진입점과 advanced composition primitives.                             |
| 0058      | 릴리즈 publisher 권한 경계         | 동일 저장소 target과 최소 권한 mutation topology.                              |
| 0059      | 공개 release-asset 모드 인터페이스 | 최소 user-intent `workflow_call` 표면과 fail-closed API.                       |
| 0060      | 통합 npm 공개 진입점               | 하나의 공개 API와 분리된 내부 권한 경계.                                       |
| 0061      | 중복 JSON member 거부              | 의미 검증 전 서명된 SLSA Statement를 엄격히 파싱.                              |
| 0062      | 신뢰 producer 정책 교집합          | Manifest와 명시적 verifier policy 충돌 처리.                                   |
| 0063      | Yarn Berry v4+ 지원 경계           | Yarn 지원을 위한 Corepack과 `packageManager` 요구사항.                         |
| 0064      | npm provenance subject 호환성      | npm PURL subject와 SHA-512 및 SHA-256 tarball digest.                          |
| 0065      | ADR 생애주기 메타데이터            | 닫힌 status 문법과 relations 필드.                                             |
| 0066      | 릴리즈 mutation 런 소유권          | 잡 클래스 concurrency와 직렬화된 mutation segment.                             |
| 0067      | 반복 run 수렴                      | Run-identity 멱등성, 결과 상태, binding 증명.                                  |
| 0068      | 검증자 신원 바인딩                 | 검증을 위한 불변 builder 및 source 신원.                                       |
| 0069      | 투명성·trust root 정책             | Rekor inclusion, 오프라인 검증, trust root 거버넌스.                           |
| 0070–0071 | Provenance 빌드 환경 기록          | 패키지 매니저 배포본, runner image, builder 필드.                              |
| 0072      | 릴리즈 에셋 run 소유권 바인딩      | Sidecar-first pair 바인딩과 custody 비귀속 선언.                               |
| 0073      | npm same-run attestation 바인딩    | npm 수렴에 published attestation run identity 필수화.                          |
| 0074      | Mutation segment 원자성            | 단일 job segment와 탐지 기반 cross-run 안전.                                   |
| 0075      | Mutation 큐 정책                   | Mutation segment contender의 `queue: max` FIFO 대기.                           |
| 0076      | Preflight와 첫 mutation 분류       | 관측 preflight와 probe 부재 표면의 첫 mutation 분류.                           |
| 0077      | Windlass provenance 서명 어댑터    | 모든 프로필의 정확한 바이트를 위한 Go-native DSSE 서명.                        |
| 0078      | pnpm 설정 전용 루트 패키지 모드    | `packages` 누락은 잘못된 데이터가 아니라 루트 전용 모드.                       |
| 0079      | 호출자 지정 빌드 소스 ref          | 모든 producer profile의 기본 릴리즈 재시도 경로인 태그 전용 `source-ref` 입력. |
| 0080      | 인증서 클레임 = 호출 컨텍스트      | 소스 정책은 서명된 provenance 필드에 바인딩; cert 소스 클레임은 호출을 증명.   |
| 0081      | npm OIDC exchange 응답 계약        | 성공 본문을 실측 형태에 핀; exchange 토큰 수명은 15분.                         |
| 0082      | publish 단계 npm 버전 핀           | 무결성 검증된 publish npm 프로비저닝 + 검토 allowlist.                         |

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

| ADR  | 결정                                                                                                | 명세 매핑                                                                                                                                                                           |
| ---- | --------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0000 | Use Markdown Architectural Decision Records                                                         | 프로세스; 런타임 명세 불필요                                                                                                                                                        |
| 0001 | Start slsa-builder as a clean repository                                                            | 기반 결정; 런타임 명세 불필요                                                                                                                                                       |
| 0002 | Extensible trusted reusable workflow foundation                                                     | Core profile contract, SLSA provenance, verification policy                                                                                                                         |
| 0003 | Thin core with profile-owned reusable workflows                                                     | Core profile contract, SLSA provenance, verification policy                                                                                                                         |
| 0004 | Go as primary implementation language                                                               | Core profile contract                                                                                                                                                               |
| 0005 | Dedicated linter toolchain                                                                          | 도구 전용                                                                                                                                                                           |
| 0006 | golangci-lint as Go linter runner                                                                   | 도구 전용                                                                                                                                                                           |
| 0007 | ShellCheck for shell glue                                                                           | 도구 전용                                                                                                                                                                           |
| 0008 | Dedicated formatters                                                                                | 도구 전용                                                                                                                                                                           |
| 0009 | Node.js as development tool runtime                                                                 | 도구 전용; core contract가 신뢰 로직 경계를 설명                                                                                                                                    |
| 0010 | pnpm for Node.js development tooling                                                                | 도구 전용                                                                                                                                                                           |
| 0011 | Lefthook for local git hook orchestration                                                           | 도구 전용                                                                                                                                                                           |
| 0012 | mise as unified development-tool runtime                                                            | 도구 전용                                                                                                                                                                           |
| 0013 | Scope initial JS/TS profile to npm packages                                                         | JS/TS npm profile, composition spec                                                                                                                                                 |
| 0014 | Support npm, pnpm, and Yarn for initial build stages                                                | JS/TS npm build and pack                                                                                                                                                            |
| 0015 | Manifest-first package manager selection                                                            | JS/TS npm build and pack                                                                                                                                                            |
| 0016 | Corepack for pnpm and Yarn build stages                                                             | JS/TS npm build and pack                                                                                                                                                            |
| 0017 | Explicit package manager version enforcement                                                        | JS/TS npm build and pack                                                                                                                                                            |
| 0018 | Publish one JS/TS package per profile run                                                           | JS/TS npm profile, build and pack                                                                                                                                                   |
| 0019 | Validate package metadata through packed artifacts                                                  | JS/TS npm build and pack                                                                                                                                                            |
| 0022 | `js-ts-npm-package-slsa3.yml` workflow entrypoint                                                   | JS/TS npm profile                                                                                                                                                                   |
| 0023 | `package-directory` as required package selector                                                    | JS/TS npm profile, build and pack                                                                                                                                                   |
| 0024 | OIDC trusted publishing without publish secrets                                                     | JS/TS npm profile, provenance and publish                                                                                                                                           |
| 0025 | Return package identity and tarball digest outputs                                                  | JS/TS npm provenance and publish                                                                                                                                                    |
| 0026 | Document supported release caller patterns and runtime guards                                       | JS/TS npm profile                                                                                                                                                                   |
| 0027 | GitHub-Hosted Ubuntu 24.04 and Node.js 24 runtime                                                   | JS/TS npm profile, build and pack                                                                                                                                                   |
| 0028 | SHA-pinned reusable workflow builder identity                                                       | Identity and build types, common provenance, release manifest, verification policy                                                                                                  |
| 0029 | Windlass-generated SLSA provenance for npm publish                                                  | Common provenance, JS/TS npm provenance and publish, verification policy                                                                                                            |
| 0030 | Accept registry URL while guaranteeing only npmjs semantics                                         | JS/TS npm profile, provenance and publish, verification policy                                                                                                                      |
| 0031 | Sigstore-signed in-toto release manifest                                                            | Identity and build types, release manifest                                                                                                                                          |
| 0032 | Constrain manual dispatch releases to version tags                                                  | JS/TS npm profile                                                                                                                                                                   |
| 0033 | Run build script only when declared                                                                 | JS/TS npm build and pack                                                                                                                                                            |
| 0034 | Do not support private dependency credentials                                                       | JS/TS npm profile                                                                                                                                                                   |
| 0035 | `actions/attest` as initial Sigstore signing adapter                                                | Core profile contract, common provenance, JS/TS npm provenance and publish, release manifest                                                                                        |
| 0036 | Three-job digest-verified publish graph                                                             | JS/TS npm provenance and publish, verification policy                                                                                                                               |
| 0037 | Define initial verification deliverables                                                            | Verification policy and fixtures                                                                                                                                                    |
| 0039 | Scope release asset profile to one asset per run                                                    | GitHub Release asset publisher                                                                                                                                                      |
| 0042 | Use acquired domains for buildType URIs                                                             | Core profile contract, identity and build types                                                                                                                                     |
| 0043 | Upload release assets to existing releases                                                          | GitHub Release asset publisher                                                                                                                                                      |
| 0045 | Use release asset name as SLSA subject name                                                         | GitHub Release asset publisher                                                                                                                                                      |
| 0046 | Keep checksums and SBOMs out of subject digest                                                      | GitHub Release asset publisher                                                                                                                                                      |
| 0048 | Make linked artifacts storage records explicit opt-in                                               | GitHub Release asset publisher                                                                                                                                                      |
| 0049 | Separate artifact production from GitHub Release asset publication                                  | Identity and build types, GitHub Release asset publisher, verification policy                                                                                                       |
| 0050 | Define producer-to-publisher handoff contract                                                       | GitHub Release asset publisher, verification policy                                                                                                                                 |
| 0051 | Distribute producer provenance with release assets                                                  | GitHub Release asset publisher, verification policy                                                                                                                                 |
| 0052 | Compose npm package tarball producer with release asset publisher                                   | Composition spec, verification policy                                                                                                                                               |
| 0053 | Three-job release manifest signing boundary                                                         | Release manifest                                                                                                                                                                    |
| 0054 | Use `slsa-builder.dev` release manifest predicate URI                                               | Release manifest, verification policy                                                                                                                                               |
| 0055 | `actions/attest` custom mode for Statement construction                                             | Common provenance, JS/TS npm provenance and publish                                                                                                                                 |
| 0056 | Treat non-selected lockfiles as stale diagnostics                                                   | JS/TS npm build and pack, JS/TS npm provenance and publish, verification policy                                                                                                     |
| 0057 | Provide public npm release-asset mode                                                               | Composition spec, JS/TS npm provenance and publish, GitHub Release asset publisher                                                                                                  |
| 0058 | Define GitHub Release asset publisher authority boundary                                            | GitHub Release asset publisher, composition spec, verification policy                                                                                                               |
| 0059 | Define public npm release-asset mode interface                                                      | Composition spec, JS/TS npm provenance and publish, GitHub Release asset publisher                                                                                                  |
| 0060 | Unify npm profile public entrypoint with release-asset mode                                         | JS/TS npm profile, composition spec, verification policy                                                                                                                            |
| 0061 | Reject duplicate JSON members in signed SLSA Statements                                             | Common provenance, JS/TS npm provenance and publish, verification policy                                                                                                            |
| 0062 | Intersect trusted producer policies                                                                 | Release manifest, GitHub Release asset publisher, verification policy                                                                                                               |
| 0063 | Limit Yarn support to Berry v4 with Corepack metadata                                               | JS/TS npm build and pack                                                                                                                                                            |
| 0064 | Use npm PURL subject with SHA-512 and SHA-256 digests                                               | Common provenance, JS/TS npm specs, composition and publisher specs, verification policy                                                                                            |
| 0065 | Use a closed status grammar and a separate relations field                                          | 프로세스; 런타임 명세 불필요                                                                                                                                                        |
| 0066 | Serialize release mutations with job-class concurrency                                              | GitHub Release asset publisher, JS/TS npm provenance and publish, release manifest, JS/TS npm package profile, verification policy                                                  |
| 0067 | Converge repeated runs within run identity                                                          | Release manifest, GitHub Release asset publisher, JS/TS npm provenance and publish, verification policy                                                                             |
| 0068 | Bind verification to immutable builder and source identities                                        | Verification policy, identity and build types, release manifest, JS/TS npm provenance and publish                                                                                   |
| 0069 | Require Rekor transparency and govern the Sigstore trust root                                       | Verification policy, common provenance, release manifest                                                                                                                            |
| 0070 | Record package manager distributions and runner image in resolvedDependencies                       | SLSA provenance v1, JS/TS npm provenance and publish, JS/TS npm build and pack, verification policy and fixtures                                                                    |
| 0071 | Activate builder.version and builderDependencies for platform components                            | SLSA provenance v1, JS/TS npm provenance and publish, verification policy and fixtures                                                                                              |
| 0072 | Use sidecar-first pair binding for release asset run ownership                                      | GitHub Release asset publisher, npm-to-release-asset composition, verification policy and fixtures                                                                                  |
| 0073 | Require published-attestation run identity for npm same-run convergence                             | JS/TS npm provenance and publish, verification policy and fixtures                                                                                                                  |
| 0074 | Use single-job mutation segments with detection-based cross-run safety                              | GitHub Release asset publisher, release manifest, npm-to-release-asset composition, verification policy and fixtures                                                                |
| 0075 | Queue mutation segment contenders with queue: max                                                   | GitHub Release asset publisher, JS/TS npm provenance and publish, release manifest, verification policy and fixtures                                                                |
| 0076 | Use observation preflights and first-mutation classification                                        | GitHub Release asset publisher, JS/TS npm package profile, verification policy and fixtures                                                                                         |
| 0077 | Use a Go-native Sigstore DSSE signer for Windlass provenance signing                                | Core profile contract, SLSA provenance v1, JS/TS npm package profile, JS/TS npm provenance and publish, release asset publisher, release manifest, composition, verification policy |
| 0078 | Treat settings-only pnpm-workspace.yaml as standalone root package mode                             | JS/TS npm build and pack, verification policy and fixtures                                                                                                                          |
| 0079 | Support a tags-only caller-specified build source ref for release retries across profiles           | Core profile contract, JS/TS npm package profile, JS/TS npm provenance and publish, verification policy and fixtures                                                                |
| 0080 | Bind source identity policy to signed provenance fields, certificate claims as invocation context   | Core profile contract, SLSA provenance v1, identity and build types, JS/TS npm provenance and publish, verification policy and fixtures                                             |
| 0081 | Pin the npm OIDC exchange response contract to the observed shape, correct token lifetime           | JS/TS npm package profile, JS/TS npm provenance and publish, verification policy and fixtures                                                                                       |
| 0082 | Pin the publish-stage npm CLI version with integrity-verified provisioning and a reviewed allowlist | JS/TS npm package profile, JS/TS npm provenance and publish, verification policy and fixtures                                                                                       |

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
- [ ] ADR이 MADR 4.0.0 구조와 Human Era 날짜 형식을 사용합니다.
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
