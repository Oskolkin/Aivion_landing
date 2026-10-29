// Package chat implements backend foundation for AI Chat MVP.
//
// ChatGPT components do not access the database directly. The backend owns
// all reads/writes and enforces data boundaries.
//
// High-level chat flow:
//   - planner builds a tool plan
//   - backend validates the plan
//   - backend executes allowlisted read-only tools
//   - backend assembles fact context
//   - answerer generates final answer
//   - backend validates the answer and stores trace
package chat
