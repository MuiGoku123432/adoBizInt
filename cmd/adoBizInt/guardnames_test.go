package main

import writeguard "sentinovo.ai/bizInt/internal/guard"

// The field vocabulary moved to internal/guard with the guarded-write body
// (GUARD-01). These aliases keep the locked CLI tests compiling unchanged
// (GUARD-06); they live in a test file because only those tests use them.
const (
	fieldState          = writeguard.FieldState
	fieldRemainingWork  = writeguard.FieldRemainingWork
	fieldCompletedWork  = writeguard.FieldCompletedWork
	fieldReviewerVote   = writeguard.FieldReviewerVote
	fieldPRStatus       = writeguard.FieldPRStatus
	fieldApprovalStatus = writeguard.FieldApprovalStatus
	fieldQueuedBuild    = writeguard.FieldQueuedBuild
	voteApproved        = writeguard.VoteApproved
)
