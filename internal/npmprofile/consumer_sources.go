package npmprofile

import "github.com/windlasstech/slsa-builder/internal/provenance"

// StatementSources keeps certificate invocation identity separate from release-policy identity.
type StatementSources struct {
	Repository       string
	BuiltRef         string
	BuiltCommit      string
	InvocationRef    string
	InvocationCommit string
}

func ResolveStatementSources(statement provenance.Statement) (StatementSources, error) {
	parameters, err := DecodeExternalParameters(statement.Predicate.BuildDefinition.ExternalParameters)
	if err != nil {
		return StatementSources{}, err
	}
	dependencies, err := finalDependencySet(statement.Predicate.BuildDefinition.ResolvedDependencies)
	if err != nil {
		return StatementSources{}, err
	}
	for _, name := range []string{"invocation-source", "built-source"} {
		if err := validateSourceDescriptor(dependencies[name], parameters); err != nil {
			return StatementSources{}, err
		}
	}
	invocation := dependencies["invocation-source"].Digest["gitCommit"]
	built := dependencies["built-source"].Digest["gitCommit"]
	if parameters.Source.Ref == "" && invocation != built {
		return StatementSources{}, npmValidationError(IDBuiltSource, "resolvedDependencies.built-source", "omitted selector requires equal source commits")
	}
	return StatementSources{Repository: parameters.Workflow.Repository, BuiltRef: effectiveBuiltRef(parameters), BuiltCommit: built, InvocationRef: parameters.Workflow.Ref, InvocationCommit: invocation}, nil
}
