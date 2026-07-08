# 아키텍처 명세

<div align="center">

[English](README.md) | 한국어

</div>

`slsa-builder`의 아키텍처 명세 모음입니다. 이 디렉터리는 관찰 가능한 동작, 계약, 스키마, 검증 기준
등 시스템이 **무엇(what)**을 수행하는지를 담습니다. **왜(why)**에 대한 내용은 `docs/decisions/`에
있는 아키텍처 결정 기록(ADR)에 담겨 있으며, 이 명세 모음은 그 ADR들을 바탕으로 작성되었습니다.

> [!Note]  
> 이 프로젝트는 명세 주도 개발(Spec-Driven Development, SDD)을 따릅니다. 먼저 ADR로 결정하고,
> 여기에서 정확한 동작을 명세한 뒤, 그 명세에 따라 구현합니다.

## SDD 모델

| 계층 | 위치                 | 답하는 질문                  | 포함할 수 있는 것                  |
| ---- | -------------------- | ---------------------------- | ---------------------------------- |
| 결정 | `docs/decisions/`    | 왜 이것을 선택했는가?        | 근거, 트레이드오프, 결과           |
| 명세 | `docs/architecture/` | 시스템이 무엇을 해야 하는가? | 계약, 스키마, 불변식, 예시, 픽스처 |
| 구현 | 소스 트리            | 어떻게 동작하는가?           | 코드, 워크플로우, 테스트, 픽스처   |

구현은 앞서 채택한 ADR과 모순되어서는 안 됩니다. 명세 초안 작성 중 기존에 채택한 ADR과 모순이
발견되거나 설계 변경이 필요할 경우, ADR 본문을 수정하지 말고 상태 업데이트 후 새 ADR을 작성하세요.

## 명세 목록

| 명세                                                                        | 목적                                                  | 근거 ADR                                                   |
| --------------------------------------------------------------------------- | ----------------------------------------------------- | ---------------------------------------------------------- |
| [Core profile contract](core-profile-contract.md)                           | Thin core와 profile-owned reusable workflow의 경계    | 0002, 0003, 0004, 0035, 0042                               |
| [Identity and build types](identity-and-buildtypes.md)                      | `builder.id`, `buildType` URI, 릴리즈 메타데이터 연결 | 0028, 0031, 0042, 0049, 0053                               |
| [SLSA provenance v1](slsa-provenance-v1.md)                                 | 공통 in-toto Statement와 SLSA v1 predicate 계약       | 0002, 0003, 0028, 0029, 0035, 0037, 0042                   |
| [JS/TS npm package profile](js-ts-npm-package-profile.md)                   | npm 패키지용 공개 reusable workflow 계약              | 0013, 0018, 0022, 0023, 0024, 0026, 0027, 0030, 0032, 0034 |
| [JS/TS npm build and pack](js-ts-npm-build-pack.md)                         | 패키지 선택, 패키지 매니저 규칙, install/build/pack   | 0014, 0015, 0016, 0017, 0018, 0019, 0023, 0027, 0033       |
| [JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md)         | 출처, 3단계 publish 그래프, npm publish               | 0024, 0025, 0028, 0029, 0030, 0035, 0036, 0037, 0052       |
| [GitHub Release asset publisher](github-release-asset-publisher.md)         | 검증된 배포자(distributor) publisher 계약             | 0039, 0043, 0045, 0046, 0048, 0049, 0050, 0051, 0052       |
| [Composed workflow internal handoff](composed-workflow-internal-handoff.md) | 같은 실행 내 producer-to-publisher 내부 핸드오프      | 0036, 0050, 0052                                           |
| [npm-to-release-asset composition](npm-to-release-asset-composition.md)     | 첫 번째 producer-to-publisher 조합                    | 0013–0037, 0049, 0050, 0051, 0052                          |
| [Release manifest](release-manifest.md)                                     | 서명된 릴리즈 매니페스트와 3단계 서명 경계            | 0028, 0031, 0035, 0042, 0053, 0054                         |
| [Verification policy and fixtures](verification-policy-and-fixtures.md)     | 검증자 정책, 픽스처 분류, 참조 명령어                 | 0028, 0029, 0030, 0036, 0037, 0049, 0050, 0051, 0052–0054  |

## ADR 추적성

모든 ADR은 명세에 대응되거나, 개발 도구 전용 혹은 대체됨(superseded), 폐기됨(deprecated) 등으로
분류됩니다. 대체되거나 폐기된 ADR은 과거 맥락일 뿐 새로운 명세나 구현을 주도해서는 안 됩니다.

자세한 ADR 추적성 표는 [`../decisions/README.ko.md`](../decisions/README.ko.md#adr-추적성)를
참조하세요.

## 명세 작성 규칙

1. **파일 하나당 명세 하나.** 각 파일은 하나의 계약에 집중합니다.
2. **ADR 참조 우선.** 모든 규범적 섹션은 하나 이상의 채택된 ADR로 추적되어야 합니다. 대체 혹은
   폐기된 ADR은 역사적 맥락으로만 인용할 수 있습니다.
3. **스키마와 예시.** 모든 입력, 출력, 불변식은 구체적인 예시와 최소 하나의 잘못된 예시를 포함해야
   합니다.
4. **구현 세부사항 금지.** 내부 코드 구조, 변수 이름, 라이브러리 선택을 설명하지 마세요. 명세는 관찰
   가능한 동작을 기술합니다.
5. **실패 명시.** 모든 "must"는 조건이 충족되지 않을 때의 실패 동작을 포함해야 합니다.
6. **TDD 아티팩트.** 각 명세는 구현에서 이를 증명할 테스트 픽스처와 검토 체크리스트를 식별해야
   합니다.
7. **교차 링크.** 관련 명세는 상대 경로로 연결하세요. 파일 간 규범적 텍스트 중복을 피하세요.
8. **용어.** 이 인덱스에서 정의한 표준 용어를 사용하세요. 정의 없이 유의어를 도입하지 마세요.

## 표준 용어

| 용어                 | 의미                                                                                                                                                 |
| -------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Core**             | 공유된 신뢰 기반: reusable workflow 불변식, 출처 생성, 다이제스트 검증 핸드오프, 서명 어댑터 인터페이스, 검증 문서화 규칙                            |
| **Profile**          | 생태계별 build, pack, subject, digest, publish, 검증 계약을 정의하는 profile-owned reusable workflow                                                 |
| **Producer**         | 최종 아티팩트 바이트와 source-to-artifact SLSA 출처를 생성하는 profile                                                                               |
| **Publisher**        | GitHub Release asset publisher profile. 아티팩트를 빌드한다고 주장하지 않고 검증하고 배포함                                                          |
| **Builder**          | source-to-artifact 출처를 생성하는 신뢰할 수 있는 reusable workflow 플랫폼. GitHub Release asset publisher는 기본 production path에서 builder가 아님 |
| **Handoff**          | producer profile과 publisher 간의 다이제스트 및 출처 검증 계약                                                                                       |
| **Sidecar**          | 주 에셋과 함께 이동하지만 주 에셋의 SLSA subject digest에 포함되지 않는 release asset                                                                |
| **Release manifest** | 릴리즈 버전을 workflow SHA, `builder.id`, `buildType` URI에 매핑하는, Windlass가 서명한 기계적으로 검증 가능한 문서                                  |

## 새 아키텍처 명세 추가 방법

1. 새 명세에 기존 채택된 ADR로 다루지 않는 아키텍처 결정이 필요하다면 새 ADR을 작성하세요.
2. 위 인벤토리 표와 [`../decisions/README.ko.md`](../decisions/README.ko.md#adr-추적성)의 ADR 추적성
   표를 업데이트하세요.
3. 관련 명세의 교차 링크를 업데이트하세요.
4. 아래 검증 체크리스트에 따라 명세를 추가하세요.
5. 제출 전 문서 품질 검사 명령을 실행하세요.

## 문서 PR 검증 체크리스트

- [ ] 모든 채택된 ADR이 명세 또는 툴링 전용으로 명시적으로 매핑되었습니다.
- [ ] 모든 폐기된 ADR이 역사적 표에 있으며 현재 요구사항으로 다루지 않습니다.
- [ ] 모든 새 명세는 목적, 근거 ADR, 대상 독자, 섹션, 입력/출력/스키마, 검증 기준을 포함합니다.
- [ ] 모든 규범적 "must"는 실패 동작을 포함합니다.
- [ ] 모든 스키마는 유효한 예시와 유효하지 않은 예시를 하나 이상 포함합니다.
- [ ] 명세 간 교차 링크는 상대 경로를 사용하고 유효합니다.
- [ ] 표준 용어가 준수됩니다.
- [ ] 린팅 및 포맷팅을 통과합니다.

  ```bash
  pnpm exec prettier --check "docs/**/*.md"
  pnpm exec markdownlint-cli2 "docs/**/*.md"
  ```
