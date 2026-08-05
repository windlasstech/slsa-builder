# Runtime Delivery Platform Spike Fixtures

> [!CAUTION] These files are non-production evidence copies from the private F01 conformance spike.
> They expose a caller-controlled `delivery_sha` only to inject a negative test and must not be
> installed as production workflows.

The fixtures record the workflows and report schema used for the 12026-08-05 live spike documented
in [`docs/implementation-runtime.md`](../../../docs/implementation-runtime.md). The private
conformance repository retains the executable Go spike payload and the run artifacts. Production
implementation must remove the `delivery_sha` input and must always check out
`${{ job.workflow_repository }}` at `${{ job.workflow_sha }}`.
