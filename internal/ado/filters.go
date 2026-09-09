package ado

// Filter predicates promoted out of the view packages (CLI-06, D-17). Bodies
// land in the GREEN commit; these stubs exist so the RED test fails on its
// assertions rather than on the build.

func IsDoneState(state string) bool { return false }

func IsTaskType(itemType string) bool { return false }

func MatchesIdentity(email, identity string) bool { return false }

func MatchesProject(project string, projects []string) bool { return false }

func MatchesRepository(repository string, repositories []string) bool { return false }
