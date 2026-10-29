package graders

// Package graders holds offline eval graders. Implementations live in package eval
// to avoid an import cycle with the harness runner.
const DeterministicVersion = "deterministic_v1"
const SemanticVersion = "semantic_rubric_v1"
const LLMGraderKey = "offline_llm_rubric"
const LLMGraderVersion = "llm_grader_v1"
