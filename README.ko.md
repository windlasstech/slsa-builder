<div align="center">

<h1>
  <a href="https://slsa-builder.dev">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="assets/logo/logo-dark.svg" />
      <img src="assets/logo/logo.svg" alt="slsa-builder" width="256" height="256" />
    </picture>
    <br>
    slsa-builder
  </a>
</h1>

[![GitHub License](https://img.shields.io/github/license/windlasstech/slsa-builder)](LICENSE)
[![SemVer Versioning](https://img.shields.io/badge/version_scheme-SemVer-0097a7)](https://semver.org/)
[![SLSA Build L3](slsa-build-l3-badge.svg)](https://slsa.dev/spec/v1.2/build-requirements#build-platform)
[![GitHub Release](https://img.shields.io/github/v/release/windlasstech/slsa-builder)](https://github.com/windlasstech/slsa-builder/releases)
[![GitHub Release Date](https://img.shields.io/github/release-date/windlasstech/slsa-builder)](https://github.com/windlasstech/slsa-builder/releases)
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-3.0-4baaaa.svg)](https://github.com/windlasstech/.github/blob/main/CODE_OF_CONDUCT.md)
[![GitHub issues](https://img.shields.io/badge/issue_tracking-GitHub-blue.svg)](https://github.com/windlasstech/slsa-builder/issues)

[![Lint](https://github.com/windlasstech/slsa-builder/actions/workflows/lint.yml/badge.svg)](https://github.com/windlasstech/slsa-builder/actions/workflows/lint.yml)
[![CodeQL](https://github.com/windlasstech/slsa-builder/actions/workflows/github-code-scanning/codeql/badge.svg)](https://github.com/windlasstech/slsa-builder/actions/workflows/github-code-scanning/codeql)
[![OSV Scanner](https://github.com/windlasstech/slsa-builder/actions/workflows/osv-scanner.yml/badge.svg)](https://github.com/windlasstech/slsa-builder/actions/workflows/osv-scanner.yml)
[![Dependency Review](https://github.com/windlasstech/slsa-builder/actions/workflows/dependency-review.yml/badge.svg)](https://github.com/windlasstech/slsa-builder/actions/workflows/dependency-review.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/windlasstech/slsa-builder/badge)](https://scorecard.dev/viewer/?uri=github.com/windlasstech/slsa-builder)

[English](README.md) | 한국어

</div>

재사용성과 프로파일 기반 확장성을 갖춘,
[SLSA 출처 증명(provenance)](https://slsa.dev/spec/v1.2/build-provenance) 빌더 기반입니다.

## 목차

- [목적과 목표](#목적과-목표)
  - [SLSA란 무엇인가요?](#slsa란-무엇인가요)
  - [출처 증명(provenance)이란 무엇인가요?](#출처-증명provenance이란-무엇인가요)
  - [대상 사용자](#대상-사용자)
- [작동 방식](#작동-방식)
- [slsa-builder를 선택해야 하는 이유](#slsa-builder를-선택해야-하는-이유)
  - [기존 대안들의 한계](#기존-대안들의-한계)
  - [slsa-builder의 강점](#slsa-builder의-강점)
- [기능](#기능)
  - [출처 증명 발급 및 게시](#출처-증명-발급-및-게시)
  - [릴리스 에셋 모드](#릴리스-에셋-모드)
  - [출처 증명 검증](#출처-증명-검증)
- [보안 및 신뢰 모델](#보안-및-신뢰-모델)
- [프로젝트 상태 및 범위](#프로젝트-상태-및-범위)
- [널리 퍼뜨려주세요](#널리-퍼뜨려주세요)
- [기술 명세 및 아키텍처 의사결정 기록](#기술-명세-및-아키텍처-의사결정-기록)
- [개발 환경 설정](#개발-환경-설정)
- [기여자들](#기여자들)
- [라이선스](#라이선스)

## 목적과 목표

slsa-builder는 작고 감사 가능한 신뢰 컴퓨팅 기반(TCB)을 갖춘, 신뢰할 수 있는
[SLSA 출처 증명(provenance)](https://slsa.dev/spec/v1.2/build-provenance) 빌더를 제공하는 깔끔하고
현대적이며 프로파일 확장 가능한 기반입니다. 기존
[slsa-github-generator](https://github.com/slsa-framework/slsa-github-generator)의 정신적
후속작으로서 기본 취지나 철학을 계승하되, 레거시 표면을 물려받지 않고 다양한 언어/저장소 생태계를
위한 SLSA Build L3+ 수준의 릴리스 워크플로를 SLSA v1.2 최신 스펙 기준으로 새롭게 구현 및 지원하는
것이 목표입니다.

slsa-builder는
[Windlass의 공급망 보안 정책 및 기준, 목표](https://github.com/windlasstech/.github/blob/main/SECURITY.md#supply-chain-integrity)를
충족하기 위한 프로젝트로 출발하였으며,
[Windlass 외에도 후술할 대상 사용자 모두에게 범용적으로 도움이 될 수 있습니다](#대상-사용자).

### SLSA란 무엇인가요?

**SLSA([Supply-chain Levels for Software Artifacts](https://slsa.dev/), "살사")**는 보안
프레임워크이자, 위변조를 방지하고 무결성을 향상시키며 패키지 및 인프라를 보호하기 위한 표준 및 통제
조치 목록입니다. 이는 공급망의 어느 단계에서든 '충분히 안전한' 것을 넘어 가능한 한 높은 복원력을
확보할 수 있도록 하기 위한 수단입니다.

> Supply-chain Levels for Software Artifacts, or SLSA (“salsa”), is a set of incrementally adoptable
> guidelines for supply chain security, established by industry consensus. The specification set by
> SLSA is useful for both software producers and consumers: producers can follow SLSA’s guidelines
> to make their software supply chain more secure, and consumers can use SLSA to make decisions
> about whether to trust a software package.
>
> ― <https://slsa.dev/spec/v1.2/about>

[SLSA는 트랙과 레벨이라는 개념으로 설명됩니다](https://slsa.dev/spec/v1.2/about#how-slsa-works).
SLSA의 각 트랙은 공급망의 특정한 한 측면에 초점을 두며, v1.2 기준
[빌드 트랙(Build Track)](https://slsa.dev/spec/v1.2/tracks#build-track)과
[소스 트랙(Source Track)](https://slsa.dev/spec/v1.2/tracks#source-track)이 있습니다.

각 트랙 내에서 레벨이 높을수록 보안 수준이 강력함을 나타냅니다. 높은 레벨일수록 공급망 위협에 대한
보다 높은 수준의 방어력을 보장하나, 구현 비용도 증가합니다. 낮은 SLSA 레벨은 도입이 용이하도록
설계되었으나, 보장하는 보안 수준이 제한적입니다. SLSA 0는 때때로, 아직 어떠한 SLSA 레벨 기준도
충족하지 못한 소프트웨어를 지칭하여 사용할 수 있습니다. 현재 SLSA 빌드 트랙은
[Build L1부터 L3까지로 구성되어 있으나](https://slsa.dev/spec/v1.2/build-track-basics),
[후속 개정에서 더 높은 레벨을 추가할 예정이라고 SLSA 공식 페이지는 밝히고 있습니다](https://slsa.dev/spec/v1.2/future-directions).

트랙과 레벨을 조합함으로써 소프트웨어가 특정 요구 사항들을 충족하는지 여부를 간단하게 설명할 수
있습니다. **특정 아티팩트가 SLSA Build L3를 충족함은 곧, 해당 소프트웨어 아티팩트가 업계 전문가들이
특정 공급망 침해를 방어하는 데 효과적이라고 인정하는 일련의 보안 관행을 준수하여 구축되었음을
의미합니다**.

### 출처 증명(provenance)이란 무엇인가요?

**출처 증명(Provenance)**이란 소프트웨어 아티팩트가 어떻게 생성되었는지에 대한 정보, 즉
메타데이터입니다. 이는 사용된 소스 코드, 빌드 시스템, 빌드 단계뿐만 아니라 빌드를 시작한 주체와
이유에 대한 정보까지 포함할 수 있습니다. 출처 증명은 사용자가 사용하는 소프트웨어 아티팩트의
진위성과 신뢰성을 판단하는 데 활용할 수 있습니다.

SLSA는 이러한 메타데이터를 기록하는 데 사용할 수 있는
[출처 증명 형식](https://slsa.dev/provenance/)을 정의합니다. slsa-builder는 SLSA Build L3 기준을
충족하는 방식으로 소프트웨어 아티팩트를 빌드하고 배포하는 도구 기반이며, 여기에는
[in-toto attestation](https://github.com/in-toto/attestation) 프레임워크 및
[SLSA 빌드 출처 증명 v1 형식](https://slsa.dev/spec/v1.2/build-provenance)(`"predicateType": "https://slsa.dev/provenance/v1"`)에
따른 적절한 출처 증명을 자동으로 발급 및 배포하는 작업 역시 포함됩니다.

> [!CAUTION]  
> 12026년 5월 11일 발생한 **Mini Shai-Hulud** 공급망 공격 사례에서 확인할 수 있듯, 출처 증명과
> 서명은 안전하기 위한 필요조건이지 충분조건은 아닙니다. 출처 증명은 SLSA 프레임워크의 일부 구성
> 요소일 뿐입니다. 만약 소프트웨어 아티팩트 빌드 및 출처 증명 발급을 수행하는 빌드 플랫폼 자체가
> 공격자에게 침해당했다면, 침해당한 패키지가 암호학적으로 '유효하게 서명된' 출처 증명과 함께
> 배포되는 상황이 발생할 수 있습니다. **Mini Shai-Hulud** 사태로 침해된 패키지들의 경우 SLSA Build
> L2에 해당하는 npm의 OIDC Trusted Publishing 및 내장 출처 증명을 사용하고 있었으나, SLSA Build L3에
> 해당하는 빌드 환경 격리는 이뤄지지 않고 있었습니다.
>
> SLSA Build L2라고 해서 무의미한 것은 절대 아니며 L0나 L1보단 훨씬 안전하지만, 그럼에도 Build L2와
> L3 간에는 유의미한 격차가 존재하며 해당 격차가 무엇인지와 그로 인한 한계를 정확히 이해할 필요는
> 있습니다. 이와 관련하여 다음 글들을 확인해 보시면 도움이 될 것입니다.
>
> - [Signed doesn’t mean safe](https://arew-m.medium.com/signed-doesnt-mean-safe-7261b0763ea0)
> - [Mini Shai-Hulud: Where SLSA's Boundaries Fall](https://slsa.dev/blog/2026/05/mini-shai-hulud-what-slsa-can-and-cannot-do)

### 대상 사용자

공식 [SLSA 소개](https://slsa.dev/spec/v1.2/about) 문서에서 밝히고 있듯 SLSA는 소프트웨어 생산자,
소비자, 인프라 제공자를 대상으로 하며, 패키지를 생산, 공급 및 배포하는 누구에게나 폭넓게 도움이
됩니다. 생산자에게는 공급망 침해(supply-chain tampering) 방지, 내부자 리스크 감소, 소프트웨어가
의도대로 소비자에게 전달될 것이라는 보장과 동시에, 원활한 소통을 위한 공통 용어(vocabulary), 실천
가능한 체크리스트, [SSDF](https://csrc.nist.gov/Projects/ssdf) 준수 수준에 대한 지표, 공급자와
소비자 간 보다 명확한 기대치 공유를 통한 신뢰를 제공합니다.

slsa-builder 역시 이러한 폭넓은 SLSA 대상 사용자 모두에게 도움이 될 수 있으며, 특히 다음 중 하나
이상에 해당하는 분들께 유용할 수 있습니다.

1. 저장소 관리자, 릴리스 워크플로 작성자 및 패키지 배포 담당자.
2. 생산자와 게시자 기본 요소(primitive)를 조합하여 워크플로를 구성하는 고급 사용자.
3. 릴리스 매니페스트(release manifest), 검증 정책, 픽스처(fixture)를 사용하는 다운스트림 검증자.

## 작동 방식

얇은 신뢰 코어가 각 프로파일 소유의
[재사용 가능한 GitHub Actions 워크플로](https://docs.github.com/ko/actions/how-tos/reuse-automations/reuse-workflows)와
함께 동작합니다. 생산자 프로파일과 게시자 프로파일, 그리고 같은 프로파일 안에서도 각 작업(빌드,
서명, 게시 등)은 각기 다른 권한으로 격리하여 실행합니다. 생산자는 아티팩트와 SLSA 출처 증명을 빌드한
뒤 게시자에게 인계(handoff)하며, 게시자는 이를 검증하고 배포합니다. 인계 절차는 다이제스트(digest)와
출처 증명 검증을 포함하며, 이 과정에서 문제 식별 시 즉시 절차를 중단합니다. 서명된 릴리스
매니페스트는 릴리스 버전을 워크플로 SHA, `builder.id` 값, `buildType` URI에 매핑합니다.

이 절에서 간략히 설명한 각 계약의 정확하고 관찰 가능한 동작은 다음 기술 명세에 구체적으로 정의되어
있습니다.

- [Core profile contract](docs/architecture/core-profile-contract.md): 얇은 신뢰 코어와 프로파일
  소유의 재사용 가능 워크플로(reusable workflow) 간 경계.
- [Identity and build types](docs/architecture/identity-and-buildtypes.md): `builder.id`,
  `buildType` URI, 릴리스 메타데이터 연결.
- [SLSA provenance v1](docs/architecture/slsa-provenance-v1.md): 공통 in-toto Statement와 SLSA v1
  predicate 계약.
- [Composed workflow internal handoff](docs/architecture/composed-workflow-internal-handoff.md):
  같은 실행 내 생산자-게시자 인계.
- [GitHub Release asset publisher](docs/architecture/github-release-asset-publisher.md): 검증된
  바이트만 배포하는 게시자(publisher) 계약.
- [Release manifest](docs/architecture/release-manifest.md): 서명된 릴리스 매니페스트와 3단계 서명
  경계.

## slsa-builder를 선택해야 하는 이유

slsa-builder는 다양한 언어와 패키지 저장소 생태계의 구성원들을 대상으로 SLSA 보안 프레임워크 도입
장벽을 실질적으로 낮추고, SLSA Build L3+ 패키지 배포 관행을 보다 폭넓게 보급 및 장려함으로써 생태계
공급망 보안에 기여하고자
합니다([ADR 0001](docs/decisions/0001-start-slsa-builder-as-clean-repository.md),
[ADR 0002](docs/decisions/0002-use-extensible-trusted-reusable-workflow-foundation.md) 참고). 기존의
다른 관련 도구들도 각각의 의도된 범위 내에서 여전히 유용하나, slsa-builder는 빌드, 출처 증명, 게시,
배포, 검증을 아우르는 하나의 통합된 프로파일 소유 신뢰 계약을 낮은 도입 장벽으로 제공한다는 강점이
있습니다.

### 기존 대안들의 한계

[SLSA 시작 가이드](https://slsa.dev/how-to/get-started)는 불필요한 추가 작업을 피하려면 처음부터
가능한 가장 높은 수준에서 시작할 것을 권고합니다. GitHub Actions 플랫폼의 경우 신뢰 경계를 적절히
설계하고 구성하면 SLSA Build L3 달성이 가능하나, 기존 대안들의 경우 다음과 같은 한계가 있습니다.

- **`slsa-github-generator` 및 `slsa-verifier` (유지관리 중단):**
  - SLSA 프레임워크 팀은
    [12022년 6월 첫 릴리스 공개](https://github.com/slsa-framework/slsa-github-generator/releases/tag/v1.0.0)를
    시작으로 약 3년간
    [`slsa-github-generator`](https://github.com/slsa-framework/slsa-github-generator)와
    [`slsa-verifier`](https://github.com/slsa-framework/slsa-verifier)를 개발 및 유지관리해 왔으며,
    이는 SLSA Build L3 도입 시의 장벽을 낮추는 데 오랜 기간 크게 기여해 왔습니다.
  - 그러나
    [`slsa-verifier`의 설계상 한계로](https://github.com/slsa-framework/slsa-verifier/issues/12),
    `slsa-github-generator`에서 제공하는 빌더는 다이제스트 기반 참조를 지원하지 않으며 `@vX.Y.Z`
    형식의 태그 버전 기반 참조만이 가능합니다. 이는
    [일반적으로 통용되는 보안 모범 관행](https://docs.github.com/ko/actions/reference/security/secure-use#using-third-party-actions)
    및
    [Windlass 자체 보안 지침](https://github.com/windlasstech/.github/blob/main/docs/security/workflow-hardening.md#action-references)에
    어긋납니다.
    - 참고: [ADR 0028](./docs/decisions/0028-use-sha-pinned-reusable-workflow-builder-identity.md)
  - 또한 결정적으로, 해당 프로젝트들은
    [`slsa-github-generator` v2.1.0 릴리스](https://github.com/slsa-framework/slsa-github-generator/releases/tag/v2.1.0),
    [`slsa-verifier` v2.7.1 릴리스](https://github.com/slsa-framework/slsa-verifier/releases/tag/v2.7.1)를
    마지막으로 12025년 7월경부터 사실상 개발 및 유지관리가 중단된 상태였고,
    [12026년 8월 7일 공식적으로 유지관리 중단을 밝혔습니다](https://github.com/slsa-framework/slsa-github-generator/pull/4515).
  - 최신 SLSA 스펙 버전은 v1.2이나 slsa-github-generator가 지원하는 출처 증명 형식은
    [v0.2](https://slsa.dev/spec/v0.2/provenance) 이후로 업데이트되지 않습니다. 따라서 해당 도구
    사용은 더 이상 권장되지 않습니다.
  - slsa-github-generator는 `workflow_dispatch` 기반 릴리스를 지원하지 않습니다. dispatch 실행의
    경우 호출자가 선택한 릴리스 대상 태그를 전달할 수 없어 출처 증명에 해당 정보가 기록되지 않으며,
    따라서 dispatch로 빌드된 아티팩트는 `slsa-verifier --source-tag` 검증이 불가능합니다. 검증기는
    `--source-tag` 지원을 태그 및 릴리스 트리거로만 한정한다고 문서화했습니다. 관련 추적 이슈인
    [slsa-github-generator#1947](https://github.com/slsa-framework/slsa-github-generator/issues/1947)은
    유지관리 종료 시점까지 해결되지 않고 열린 상태였습니다. 이로 인해 지원되는 고정 파이프라인
    재시도 경로가 존재하지 않았으며, 파이프라인 결함으로 실패한 릴리스는 태그를 이동하거나
    재생성하지 않는 한 수정된 호출자 워크플로로 재실행할 수 없었습니다.
    - 참고:
      [ADR 0079](./docs/decisions/0079-support-tags-only-caller-specified-build-source-ref-for-release-retries-across-profiles.md),
      [ADR 0080](./docs/decisions/0080-bind-source-identity-policy-to-signed-provenance-fields-and-treat-certificate-source-claims-as-invocation-context.md),
      [osv-scanner#632](https://github.com/google/osv-scanner/issues/632)
- **GitHub `actions/attest`:**
  - GitHub Artifact Attestations, 즉 [`attest` 액션](https://github.com/actions/attest)을 활용하면
    GitHub Actions 플랫폼에서 SLSA Build L3 요건을 충족하면서 패키지를 빌드 및 배포하는 것이
    가능합니다.
  - GitHub Artifact Attestations는 출처 증명 발급 및 [Sigstore](https://www.sigstore.dev/) 인스턴스
    기반 서명 등의 작업을 자동으로 처리해 줍니다.
  - 단, GitHub-hosted runners에서 GitHub Artifact Attestations를 사용하는 것만으로 달성 가능한
    수준은 SLSA Build L2까지입니다. Build L3에서 요구하는
    [출처 증명 위조 방지](https://slsa.dev/spec/v1.2/build-requirements#provenance-unforgeable) 및
    [빌드 환경 격리](https://slsa.dev/spec/v1.2/build-requirements#isolated) 요건까지 충족하기
    위해서는 빌드 및 서명용으로 사용할
    [재사용 가능한 GitHub Actions 워크플로](https://docs.github.com/ko/actions/how-tos/reuse-automations/reuse-workflows)를
    따로 마련하여, 빌드 및 배포 프로세스를 호출자 워크플로(caller workflow)와는 다른 저장소의 격리된
    환경에서 실행할 수 있도록 하는 추가적인 준비 절차가 필요합니다. 이는
    [`attest` 액션](https://github.com/actions/attest) 사용만으로는 해결되지 않으며, 상대적으로 도입
    장벽을 높이는 요인입니다.
    - [GitHub Docs: Artifact attestations 개념](https://docs.github.com/ko/actions/concepts/security/artifact-attestations)
    - [GitHub Blog: GitHub Artifact Attestations로 빌드 보안 강화 및 SLSA Level 3 달성](https://github.blog/enterprise-software/devsecops/enhance-build-security-and-reach-slsa-level-3-with-github-artifact-attestations/)
    - [GitHub Docs: Artifact attestations로 보안 등급 높이기](https://docs.github.com/ko/actions/how-tos/secure-your-work/use-artifact-attestations/increase-security-rating)
  - 또한, [`attest` 액션](https://github.com/actions/attest)은 Provenance 모드에서 SHA-256
    다이제스트 출력만을 지원합니다. 많은 경우 문제가 없으나, npm처럼 SHA-512만을 입력으로 받는
    패키지 매니저/저장소의 경우 `--provenance-file` 혹은 그와 비슷한 옵션 사용이 불가능할 수
    있습니다.

### slsa-builder의 강점

- **낮은 진입 장벽의 SLSA 도입 및 Build L3 요건 충족:** 앞서 다룬 것처럼 Build L3의 출처 증명 위조
  방지 및 빌드 환경 격리 요건을 충족하려면 대상 패키지와는 별도의 저장소에 격리된 재사용 가능
  워크플로를 마련해야 하며, 이 신뢰 경계를 각 저장소 관리자가 직접 설계하고 유지하는 것은 부담으로
  다가올 수 있습니다. slsa-builder는 이 경계를 프로파일별 재사용 가능 워크플로로 직접 설계해, SLSA
  Build L3로 가는 미리 닦아 둔 길(paved road)로 제공합니다. 호출자 워크플로에서는 `uses:` 참조와
  `package-directory` 같은 소수의 입력만으로 빌드, 출처 증명 발급, 서명, 게시, 검증을 하나의
  계약으로 도입할 수 있습니다. 워크플로 참조는 태그 대신 커밋 SHA로 고정(pinned)할 수 있어 앞서
  지적한 태그 기반 참조의 한계도 피할 수 있으며, 출처 증명은 최신 SLSA v1.2 스펙을 따릅니다.
  slsa-github-generator가 보여 준 GitHub 환경에서의 낮은 진입 장벽 모델을 최신 스펙 위에서 이어가는
  것이 목표입니다
  ([ADR 0002](docs/decisions/0002-use-extensible-trusted-reusable-workflow-foundation.md),
  [ADR 0003](docs/decisions/0003-use-thin-core-with-profile-owned-reusable-workflows.md),
  [ADR 0023](docs/decisions/0023-use-package-directory-as-required-js-ts-npm-package-selector.md),
  [ADR 0028](docs/decisions/0028-use-sha-pinned-reusable-workflow-builder-identity.md); 명세:
  [JS/TS npm package profile](docs/architecture/js-ts-npm-package-profile.md)).
- **최소화한 신뢰 표면:** slsa-builder는 slsa-github-generator의 넓은 레거시 및 BYOB framework
  표면을 물려받는 대신, 완전히 새로운 출발점과 더 작고 의도적으로 선택한 신뢰 표면을 택했습니다
  ([ADR 0001](docs/decisions/0001-start-slsa-builder-as-clean-repository.md),
  [ADR 0002](docs/decisions/0002-use-extensible-trusted-reusable-workflow-foundation.md),
  [ADR 0003](docs/decisions/0003-use-thin-core-with-profile-owned-reusable-workflows.md); 명세:
  [Core profile contract](docs/architecture/core-profile-contract.md)).
- **정준 출처 증명 시맨틱:** slsa-builder는 해당하는 프로파일의 `builder.id`, `buildType`,
  `externalParameters`, subject, digest, publish, verification semantics를 기록하고, Go-native
  Sigstore DSSE signer로 정확한 Statement 바이트를 조립하고 서명합니다. 또한 한 subject 안에 동일
  tarball 바이트에 대한 SHA-256 및 SHA-512 다이제스트를 함께 담을 수 있으며, 이를 통해 각 subject에
  대한 단일 다이제스트 출력만을 지원하는 다른 도구 대비 더 뛰어난 호환성을 확보할 수 있습니다
  ([ADR 0029](docs/decisions/0029-use-windlass-generated-slsa-provenance-for-npm-publish.md),
  [ADR 0042](docs/decisions/0042-use-acquired-domains-for-buildtype-uris.md),
  [ADR 0064](docs/decisions/0064-use-npm-purl-subject-with-sha512-and-sha256.md),
  [ADR 0077](docs/decisions/0077-use-go-native-sigstore-dsse-signer-for-windlass-provenance-signing.md);
  명세: [SLSA provenance v1](docs/architecture/slsa-provenance-v1.md),
  [Identity and build types](docs/architecture/identity-and-buildtypes.md)).
- **종단 간 릴리스 신뢰:** 엄격한 signed-JSON 처리, immutable builder와 source binding, 관리되는
  trust root를 갖춘 Rekor 기반 오프라인 검증, 서명된 릴리스 매니페스트, provenance-gated 릴리스 에셋
  게시, 제어된 release mutation을 소스 빌드부터 릴리스 배포까지의 전 과정에 걸쳐 종단
  간에(end-to-end) 연결합니다
  ([ADR 0031](docs/decisions/0031-use-sigstore-signed-in-toto-release-manifest.md),
  [ADR 0049](docs/decisions/0049-separate-artifact-production-from-github-release-asset-publication.md),
  [ADR 0050](docs/decisions/0050-define-producer-to-publisher-handoff-contract.md),
  [ADR 0051](docs/decisions/0051-distribute-producer-provenance-with-release-assets.md),
  [ADR 0053](docs/decisions/0053-use-three-job-release-manifest-signing-boundary.md),
  [ADR 0061](docs/decisions/0061-reject-duplicate-json-members-in-signed-slsa-statements.md),
  [ADR 0066](docs/decisions/0066-serialize-release-mutations-with-job-class-concurrency.md),
  [ADR 0067](docs/decisions/0067-converge-repeated-runs-within-run-identity.md),
  [ADR 0068](docs/decisions/0068-bind-verification-to-immutable-builder-and-source-identities.md),
  [ADR 0069](docs/decisions/0069-require-rekor-transparency-and-govern-sigstore-trust-root.md),
  [ADR 0072](docs/decisions/0072-use-sidecar-first-pair-binding-for-release-asset-run-ownership.md),
  [ADR 0073](docs/decisions/0073-require-published-attestation-run-identity-for-npm-same-run-convergence.md),
  [ADR 0074](docs/decisions/0074-use-single-job-mutation-segments-with-detection-based-cross-run-safety.md),
  [ADR 0075](docs/decisions/0075-queue-mutation-segment-contenders-with-queue-max.md),
  [ADR 0076](docs/decisions/0076-use-observation-preflights-and-first-mutation-classification.md);
  명세: [Release manifest](docs/architecture/release-manifest.md),
  [GitHub Release asset publisher](docs/architecture/github-release-asset-publisher.md),
  [Verification policy and fixtures](docs/architecture/verification-policy-and-fixtures.md)).
- **릴리스 대상 태그 지정 및 재시도 관련 향상된 유연성:** 파이프라인 결함으로 실패한 릴리스도 수정된
  파이프라인이 있는 ref(예: `main`)에서 dispatch하여 재시도할 수 있으며, 빌드·증명 대상 콘텐츠는
  서명된 릴리스 태그로 유지됩니다. 태그 전용 선택 입력 `source-ref`는 출처 증명을 태그 커밋에
  고정하고, dispatch ref는 감사 가능한 호출 컨텍스트로 별도 기록됩니다. 전임 도구가 끝내 제공하지
  못한 격차를 해소합니다
  ([ADR 0079](docs/decisions/0079-support-tags-only-caller-specified-build-source-ref-for-release-retries-across-profiles.md),
  [ADR 0080](docs/decisions/0080-bind-source-identity-policy-to-signed-provenance-fields-and-treat-certificate-source-claims-as-invocation-context.md);
  명세: [JS/TS npm package profile](docs/architecture/js-ts-npm-package-profile.md)).

## 기능

현재 JS/TS 패키지, GitHub Releases 및 npm을 대상으로 SLSA 출처 증명의 발급과 배포, 검증을
제공합니다. 사용 중인 생태계와 배포 대상에 맞는 프로파일을 선택해 호출자 워크플로에서 참조하면
됩니다. 지원 생태계와 배포 대상을 앞으로 지속적으로 추가해 나갈 것입니다.

### 출처 증명 발급 및 게시

| 생태계    | 프로파일                                                                    | 설명                                                   | 상태       |
| :-------- | :-------------------------------------------------------------------------- | :----------------------------------------------------- | :--------- |
| JS/TS npm | [JS/TS npm package profile](docs/architecture/js-ts-npm-package-profile.md) | npm 패키지 빌드, SLSA v1 출처 증명 발급·서명, npm 게시 | 프리릴리스 |

- **각 실행당 정확히 하나의 패키지:** 필수 입력 `package-directory`로 대상 패키지를 선택합니다. 공개
  계약은 이 필수 입력 하나와 선택 입력 여덟 개로 구성됩니다.
- **고정 파이프라인 릴리스 재시도:** 태그 전용 선택 입력 `source-ref`를 사용하면, 수정된
  파이프라인이 있는 ref(예: `main`)에서 dispatch하면서도 빌드·증명 대상 콘텐츠는 서명된 릴리스
  태그로 유지하는 릴리스 재시도가 가능합니다. 태그 재생성도, 출처 증명 주장 약화도 필요 없습니다.
  출처 증명은 빌드된 태그 신원을 기록하고, dispatch ref는 호출 컨텍스트로 별도 기록합니다
  ([ADR 0079](docs/decisions/0079-support-tags-only-caller-specified-build-source-ref-for-release-retries-across-profiles.md)와
  [ADR 0080](docs/decisions/0080-bind-source-identity-policy-to-signed-provenance-fields-and-treat-certificate-source-claims-as-invocation-context.md)
  참고).
- **매니페스트 우선 패키지 매니저 선택:** npm, pnpm, Corepack을 통한 Yarn Berry v4+를 지원하며, 빌드
  스크립트는 선언된 경우에만 실행합니다
  ([JS/TS npm build and pack](docs/architecture/js-ts-npm-build-pack.md) 참고).
- **비밀 없는 신뢰 게시:** npm OIDC trusted publishing으로 인증하므로 장기 보관 publish secret이
  필요 없습니다. slsa-builder가 생성하는 SLSA v1 출처 증명은 하나의 npm Package URL subject에 동일
  tarball 바이트의 SHA-512와 SHA-256 digest를 함께 담고, Go-native Sigstore DSSE signer로 서명한 뒤
  세 job으로 구성된 publish graph를 거쳐 게시합니다
  ([JS/TS npm provenance and publish](docs/architecture/js-ts-npm-provenance-publish.md) 참고).

### 릴리스 에셋 모드

빌드한 tarball과 출처 증명 sidecar(`tarball.intoto.jsonl`)를 다이제스트 검증을 거쳐 기존 GitHub
Release에 업로드합니다
([npm-to-release-asset composition](docs/architecture/npm-to-release-asset-composition.md) 참고).

### 출처 증명 검증

다운스트림 검증자를 위해 검증 정책 스키마, fixture 분류 체계, 참조 명령을 제공합니다. 초기
프로파일에는 독립 실행형 verifier CLI가 없습니다
([Verification policy and fixtures](docs/architecture/verification-policy-and-fixtures.md) 참고).

## 보안 및 신뢰 모델

slsa-builder의 신뢰 모델은
["서명된 출처 증명은 안전의 필요조건이지 충분조건은 아니다"라는 전제](#출처-증명provenance이란-무엇인가요)에서
출발합니다. 이 섹션은 slsa-builder가 어떤 위협을 방어하는지, 어떤 외부 신뢰에 의존하는지, 그리고
방어 범위의 한계가 어디인지를 정리합니다.

### 키 관리와 서명

- **장기 보관 secret 없음:** npm 게시 인증에는 OIDC trusted publishing을 사용하므로 저장소에 publish
  token을 보관할 필요가 없습니다
  ([ADR 0024](docs/decisions/0024-use-oidc-trusted-publishing-without-publish-secrets.md)).
- **keyless 서명:** 서명에는 Sigstore(Fulcio 단기 인증서와 OIDC 워크로드 신원)를 사용하므로 개인
  키를 발급·보관·교체하는 운영 부담 자체가 없습니다. 정확한 Statement 바이트의 조립과 서명은
  Go-native Sigstore DSSE signer가 담당합니다
  ([ADR 0077](docs/decisions/0077-use-go-native-sigstore-dsse-signer-for-windlass-provenance-signing.md)).
- **엄격한 서명 바이트:** 서명되는 JSON의 중복 멤버를 거부해, 파서 차이로 서명 대상 바이트가
  갈라지는 것을 방지합니다
  ([ADR 0061](docs/decisions/0061-reject-duplicate-json-members-in-signed-slsa-statements.md)).

### 검증 모델

- **불변 신원 바인딩:** 검증은 이동 가능한 태그가 아니라 커밋 SHA로 고정된 워크플로의 `builder.id`와
  불변 source identity에 바인딩됩니다
  ([ADR 0068](docs/decisions/0068-bind-verification-to-immutable-builder-and-source-identities.md)).
  소스 기대값은 서명된 출처 증명 필드에 바인딩되고, 서명 인증서의 플랫폼 고정 소스 클레임은 호출
  컨텍스트를 인증합니다. dispatch 재시도로 다른 호출 ref에서 태그를 빌드하더라도 둘은 암호학적으로
  묶여 있습니다
  ([ADR 0080](docs/decisions/0080-bind-source-identity-policy-to-signed-provenance-fields-and-treat-certificate-source-claims-as-invocation-context.md)).
- **투명성 로그와 관리되는 trust root:** 모든 서명은 Rekor 투명성 로그에 기록되어야 하며, Sigstore
  trust root는 프로젝트가 관리하는 고정본을 사용해 오프라인 검증을 가능하게 합니다
  ([ADR 0069](docs/decisions/0069-require-rekor-transparency-and-govern-sigstore-trust-root.md)).
- **이중 다이제스트:** 하나의 subject가 동일 tarball 바이트의 SHA-512와 SHA-256을 함께 담아,
  다이제스트 알고리즘이 다른 검증 경로를 모두 지원합니다
  ([ADR 0064](docs/decisions/0064-use-npm-purl-subject-with-sha512-and-sha256.md)).
- 검증 정책 스키마, fixture 분류 체계, 참조 명령은
  [Verification policy and fixtures](docs/architecture/verification-policy-and-fixtures.md)에
  정의되어 있습니다.

### 릴리스 무결성

- **서명된 릴리스 매니페스트:** 릴리스 버전을 워크플로 SHA, `builder.id`, `buildType`에 매핑하는
  매니페스트를 3단계 서명 경계로 서명합니다
  ([ADR 0031](docs/decisions/0031-use-sigstore-signed-in-toto-release-manifest.md),
  [ADR 0053](docs/decisions/0053-use-three-job-release-manifest-signing-boundary.md)).
- **provenance-gated 게시:** 게시자는 생산자의 출처 증명과 다이제스트를 검증한 뒤에만 배포하며,
  검증되지 않은 바이트는 배포하지 않습니다
  ([ADR 0049](docs/decisions/0049-separate-artifact-production-from-github-release-asset-publication.md),
  [ADR 0050](docs/decisions/0050-define-producer-to-publisher-handoff-contract.md),
  [ADR 0051](docs/decisions/0051-distribute-producer-provenance-with-release-assets.md)).
- **제어된 release mutation:** 릴리스 변경은 job 클래스 동시성으로 직렬화하고, 같은 실행 내 반복
  실행은 동일 결과로 수렴합니다
  ([ADR 0066](docs/decisions/0066-serialize-release-mutations-with-job-class-concurrency.md),
  [ADR 0067](docs/decisions/0067-converge-repeated-runs-within-run-identity.md)).

### 신뢰 경계와 의존성

slsa-builder는 GitHub Actions(호스티드 러너와 OIDC 공급자), Sigstore(Fulcio, Rekor), npm
레지스트리를 신뢰 의존 대상으로 삼습니다. SLSA Build L3의 격리 요건은 빌드와 서명을 호출자
워크플로와 다른 저장소의 재사용 워크플로로 분리함으로써 충족합니다
([ADR 0028](docs/decisions/0028-use-sha-pinned-reusable-workflow-builder-identity.md)). 프로젝트는
SLSA Build L3를 지향합니다.

### 취약점 신고

보안 취약점은 공개 이슈가 아닌
[Windlass 보안 정책](https://github.com/windlasstech/.github/blob/main/SECURITY.md)에 안내된 경로로
신고해 주세요.

## 프로젝트 상태 및 범위

프리릴리스 상태입니다. 저장소에는 실제 Go 구현이 있지만, 프로젝트는 아직 안정 릴리스를 표방하지
않으며 첫 안정 버전 전까지 워크플로 인터페이스가 변경될 수 있습니다.

초기 비목표:

- 일반 파일 또는 컨테이너.
- 독립 실행형 verifier CLI.
- custom-token 또는 PAT를 통한 릴리스 변경.
- producer provenance가 없는 raw 파일 업로드.
- 새 npm 패키지 identity의 첫 발행.
- publish credential로 사용하는 private dependency credential.

## 널리 퍼뜨려주세요

slsa-builder에 대한 소개나 응원부터 문제점 및 개선 필요 사항에 대한 건설적인 비판까지, slsa-builder
및 SLSA 프레임워크에 대해 다루는 글이나 영상 등의 콘텐츠가 많아질수록 생태계 전반에 걸쳐 해당
프레임워크 및 도구들에 대한 인식을 제고하고 도입을 확대해 나갈 수 있을 것입니다. 기여와 피드백을
언제나
환영합니다([기여 가이드라인](https://github.com/windlasstech/.github/blob/main/CONTRIBUTING.md),
[이슈 트래커](https://github.com/windlasstech/slsa-builder/issues)).

또한, 그보다 훨씬 간단하게 slsa-builder를 도와주실 수 있는 방법도 있습니다. slsa-builder를
사용하거나 지지하는 프로젝트라면, README나 기타 프로젝트 페이지에 링크를 걸어 프로젝트를 알려
주세요. README 혹은 비슷한 성격의 프로젝트 문서에 포함할 수 있도록, 다음과 같이 종류별 배지를
제공합니다.

배지는 프로젝트 로고를 포함하는 정적 SVG로 [`assets/badges/`](assets/badges/)에 들어 있습니다.
shields.io 같은 외부 배지 서비스를 거치지 않고 본 저장소가 직접 제공하므로 전체 색상 로고를 그대로
유지하고, 배지 자산을 프로젝트와 함께 버전 관리할 수 있습니다. 아래 스니펫을 그대로 복사해
사용하거나, SVG 파일을 복사해 직접 호스팅해도 됩니다. 스니펫 URL의 `main`을 릴리스 태그 혹은 커밋
SHA로 바꾸면 불변 참조로 고정할 수 있습니다.

### built with slsa-builder

slsa-builder로 artifact를 빌드하고 배포하는 프로젝트용입니다.

[![built with slsa-builder](assets/badges/built-with-slsa-builder.svg)](https://github.com/windlasstech/slsa-builder)

Markdown:

```markdown
[![built with slsa-builder](https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/built-with-slsa-builder.svg)](https://github.com/windlasstech/slsa-builder)
```

HTML:

```html
<a href="https://github.com/windlasstech/slsa-builder"
  ><img
    src="https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/built-with-slsa-builder.svg"
    alt="built with slsa-builder"
/></a>
```

### verified with slsa-builder

slsa-builder 검증 정책과 fixture로 릴리스를 검증하는 다운스트림 검증자용입니다.

[![verified with slsa-builder](assets/badges/verified-with-slsa-builder.svg)](https://github.com/windlasstech/slsa-builder)

Markdown:

```markdown
[![verified with slsa-builder](https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/verified-with-slsa-builder.svg)](https://github.com/windlasstech/slsa-builder)
```

HTML:

```html
<a href="https://github.com/windlasstech/slsa-builder"
  ><img
    src="https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/verified-with-slsa-builder.svg"
    alt="verified with slsa-builder"
/></a>
```

### slsa-builder 로고 배지

프로젝트를 가리키는 용도로 쓸 수 있는 단일 배경 로고 배지로, 회색(기본)과 녹색 두 가지 색상으로
제공됩니다.

[![slsa-builder](assets/badges/slsa-builder.svg)](https://github.com/windlasstech/slsa-builder)
[![slsa-builder (green)](assets/badges/slsa-builder-green.svg)](https://github.com/windlasstech/slsa-builder)

Markdown:

```markdown
[![slsa-builder](https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/slsa-builder.svg)](https://github.com/windlasstech/slsa-builder)
[![slsa-builder (green)](https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/slsa-builder-green.svg)](https://github.com/windlasstech/slsa-builder)
```

HTML:

```html
<a href="https://github.com/windlasstech/slsa-builder"
  ><img
    src="https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/slsa-builder.svg"
    alt="slsa-builder"
/></a>
<a href="https://github.com/windlasstech/slsa-builder"
  ><img
    src="https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/slsa-builder-green.svg"
    alt="slsa-builder"
/></a>
```

### 스타일 변형

각 배지는 shields.io와 유사하게 네 가지 스타일로 제공됩니다. 기본 `flat`은 접미사 없이 제공되며,
나머지 스타일은 파일명에 접미사를 붙여 구분합니다.

| 스타일          | 파일명 예시                                 | 미리보기                                                                                            |
| --------------- | ------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `flat` (기본)   | `built-with-slsa-builder.svg`               | ![built with slsa-builder — flat](assets/badges/built-with-slsa-builder.svg)                        |
| `flat-square`   | `built-with-slsa-builder-flat-square.svg`   | ![built with slsa-builder — flat-square](assets/badges/built-with-slsa-builder-flat-square.svg)     |
| `plastic`       | `built-with-slsa-builder-plastic.svg`       | ![built with slsa-builder — plastic](assets/badges/built-with-slsa-builder-plastic.svg)             |
| `for-the-badge` | `built-with-slsa-builder-for-the-badge.svg` | ![built with slsa-builder — for-the-badge](assets/badges/built-with-slsa-builder-for-the-badge.svg) |

같은 접미사 규칙이 `verified-with-slsa-builder`와 `slsa-builder` 배지에도 그대로 적용됩니다. 로고
배지의 녹색 변형은 색상 접미사가 스타일 접미사보다 앞에 옵니다(예:
`slsa-builder-green-for-the-badge.svg`). 예를 들어 `for-the-badge` 스타일의 Markdown 스니펫은 다음과
같습니다.

```markdown
[![built with slsa-builder](https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/built-with-slsa-builder-for-the-badge.svg)](https://github.com/windlasstech/slsa-builder)
```

## 기술 명세 및 아키텍처 의사결정 기록

[아키텍처 색인](docs/architecture/README.ko.md)과 [ADR 색인](docs/decisions/README.ko.md)을
참고하세요. ADR은 MADR 형식으로 근거를 기록하며, 아키텍처 명세는 정확히 관찰 가능한 동작을
정의합니다.

## 개발 환경 설정

이 저장소는 개발 도구 런타임 버전을 설치하고 고정하기 위해 [mise](https://mise.jdx.dev/)를
사용합니다. Go가 주요 구현 언어이며, Node.js와 pnpm은 Prettier나 Lefthook 같은 개발 도구 용도로만
사용합니다.

### 필수 조건

- [mise](https://mise.jdx.dev/getting-started.html) 설치
- 사용자 이름과 이메일을 설정한 Git

### 부트스트랩

```bash
mise install
pnpm install
```

이 명령은 `mise.toml`에 정의된 Go, Node.js, pnpm, CLI 도구의 고정된 버전을 설치합니다. Lefthook
hook은 mise가 Lefthook을 설치할 때 `postinstall` 단계로 자동 설치됩니다. 그 후 `pnpm install`
단계에서 `package.json`에 선언된 프로젝트 로컬 개발 의존성을 설치합니다.

CI에서는 레지스트리에 대한 API 호출을 방지하기 위해 잠금 모드로 mise를 실행하세요.

```bash
MISE_LOCKED=1 mise install
pnpm install
```

부트스트랩을 완료하면 mise를 통해 다음 명령을 사용할 수 있습니다.

```bash
go version
node --version
pnpm --version
golangci-lint --version
shellcheck --version
shfmt --version
lefthook --version
actionlint --version
```

### mise와 pnpm의 담당 범위

mise는 언어 런타임과 독립 실행형 CLI 바이너리를 설치합니다.

- Go, Node.js, pnpm
- `golangci-lint`, `shellcheck`, `shfmt`, `lefthook`, `actionlint`

Go 소스 포매팅과 import 정규화는 독립 실행형 포매터 바이너리가 아닌, `.golangci.yml`에 구성된
`golangci-lint` 포매터(`gofmt`, `goimports`)가 담당합니다.

pnpm은 저장소 설정 파일과 길게 연결된 Node.js 기반 개발 의존성을 설치합니다.

- `prettier` (`.prettierrc`로 구성)
- `markdownlint-cli2` (`.markdownlint-cli2.jsonc`로 구성)

Prettier와 `markdownlint-cli2`를 프로젝트 로컬 pnpm 의존성으로 유지함으로써 `pnpm-lock.yaml`에 전체
의존성 그래프가 남고, 에디터 통합 및 조직의 의존성 검사 워크플로우와 정렬됩니다.

### 도구 버전

도구 버전은 `mise.toml`에 선언되어 있습니다. 플랫폼 간 재현 가능한 설치를 보장하기 위해 `mise.lock`
파일이 커밋되어 있습니다. `mise.toml`에서 도구 버전을 변경한 경우 다음 명령으로 잠금 파일을 다시
생성하세요.

```bash
mise lock
```

### 커밋 규약 및 서명

이 프로젝트는 모든 커밋에 `Signed-off-by:` 트레일러(DCO)가 필요합니다. Lefthook이 로컬에서 이를
강제하도록 구성되어 있으며, CI와 브랜치 보호가 권위 있는 검증을 수행합니다.

## 기여자들

이 프로젝트에 기여해 주신 모든 분께 감사드립니다. 기여자 목록은
[GitHub 기여자 그래프](https://github.com/windlasstech/slsa-builder/graphs/contributors)에서 확인할
수 있습니다.

[![Contributors](https://contrib.rocks/image?repo=windlasstech/slsa-builder)](https://github.com/windlasstech/slsa-builder/graphs/contributors)

## 라이선스

[Apache License 2.0](LICENSE)에 따라 배포됩니다.
