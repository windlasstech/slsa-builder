# 플랫폼 계약 스파이크 픽스처

> [!CAUTION] 이 파일들은 12026-08-05에 수행한 F03 비프로덕션 스파이크의 증거 사본입니다. 워크플로
> 파일은 검토 목적으로만 보관하며 프로덕션 워크플로로 설치하면 안 됩니다.

이 디렉터리는 초기 npm 프로필 구현을 차단하는 외부 플랫폼 동작을 고정합니다. 증거는 비공개 적합성
저장소의 재사용 가능 워크플로 관찰 결과와 공개 임시 저장소의 성공 실행을 결합합니다. 공개 임시
저장소는 현재 플랜에서 비공개 저장소의 GitHub 아티팩트 증명 저장소를 사용할 수 없어서 한시적으로
사용했습니다.

## 정리 상태

증거 아티팩트는 다운로드하고 정제했지만 공개 저장소는 아직 삭제하지 못했습니다. 현재 GitHub CLI
토큰에 `delete_repo` OAuth 범위가 없고 두 번의 디바이스 인증 시도는 승인 없이 시간 초과되었습니다.
저장소는 계속 임시 상태이며 해당 범위를 승인한 뒤 삭제해야 합니다. `platform-contract-report.json`은
이 상태를 `repository_deleted_after_capture: false`로 기록합니다.

## 고정된 결과

- 재사용 가능 워크플로 OIDC 토큰에는 불변 호출 워크플로를 가리키는 비어 있지 않은
  `job_workflow_ref`와 40자리 16진수 `job_workflow_sha`가 있습니다. 원본 토큰은 저장하지 않았습니다.
- `actions/attest@v4.2.2` 사용자 정의 모드는 `application/vnd.dev.sigstore.bundle.v0.3+json`
  Sigstore 번들을 `attestation.json`으로 출력합니다.
- 동일한 주체와 술어 입력으로 두 번 호출하면 전체 번들 바이트는 달랐지만 서명된 Statement 페이로드
  바이트는 같았습니다. 출력 번들은 그대로 보존해야 하며 서명 바이트의 동일성을 요구하면 안 됩니다.
- `queue: max`는 github.com에서 `cancel-in-progress: false`와 함께 실행되었습니다. GitHub는 대기
  100개 한도와 가득 찬 이후 취소를 문서화합니다. 오버플로 런타임 관찰과 GHES 동등성은 L03으로
  연기합니다.
- actionlint 1.7.12는 `queue: max`를 거부합니다. 정확한 진단과 업스트림 이슈를 증거에 기록했습니다.
- Node.js `v24.18.0`과 npm `11.16.0`은 토큰이나 인증 대체 없이
  `npm publish --dry-run --provenance-file`의 외부 Sigstore 번들을 허용했습니다.
- npm 증명 엔드포인트의 배열은 위치가 아닌 `predicateType`으로 선택해야 합니다. 게시 실행 식별자는
  인증서 OID `1.3.6.1.4.1.57264.1.21`과 `predicate.runDetails.metadata.invocationId`에 모두
  있습니다.

## GitHub SLSA 저장소 제약

GitHub 증명 저장소는 `https://slsa.dev/provenance/v1` 술어에 의미 검증을 적용하며 저장이 활성화된
경우 Windlass 사용자 정의 buildType을 거부했습니다. 성공한 어댑터 스파이크는 번들 출력과 저장소 읽기
동작을 분리해 검증하기 위해 자유 형식 사용자 정의 술어 URI를 사용했습니다.

ADR 0055는 GitHub 증명 저장소를 선택 사항으로 정의하므로 이 결과와 모순되지 않습니다. 프로덕션
서명은 필수 SLSA 술어와 원본 출력 번들을 사용해야 하지만 GitHub 저장소가 Windlass 사용자 정의
buildType을 허용한다고 가정하면 안 됩니다.

파일별 계약과 공식 출처, 재현 명령은 영문 [`README.md`](README.md)를 참고하세요.
