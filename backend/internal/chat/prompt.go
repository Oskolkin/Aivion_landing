package chat

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	PlannerPromptVersion = "stage_10_ai_chat_planner_v2"
	AnswerPromptVersion  = "stage_10_ai_chat_answer_v1"
)

func BuildPlannerSystemPrompt(tools []ToolDefinition) string {
	blocks := make([]string, 0, len(tools))
	for _, t := range tools {
		argParts := make([]string, 0, len(t.AllowedArgs))
		argNames := make([]string, 0, len(t.AllowedArgs))
		for name := range t.AllowedArgs {
			argNames = append(argNames, name)
		}
		sort.Strings(argNames)
		for _, name := range argNames {
			arg := t.AllowedArgs[name]
			required := "optional"
			if arg.Required {
				required = "required"
			}
			allowed := ""
			if len(arg.AllowedValues) > 0 {
				allowed = fmt.Sprintf(", allowed=%s", strings.Join(arg.AllowedValues, "|"))
			}
			argParts = append(argParts, fmt.Sprintf("%s(%s,%s%s)", name, arg.Type, required, allowed))
		}
		blocks = append(blocks, fmt.Sprintf(
			"- %s: %s; args=[%s]; max_limit=%d; read_only=%t; intents=%s",
			t.Name,
			t.Purpose,
			strings.Join(argParts, ", "),
			t.MaxLimit,
			t.ReadOnly,
			joinIntents(t.SupportedIntents),
		))
	}
	return "You are a planner for marketplace assistant. Use only allowlisted read-only tools.\n" +
		"Return STRICT JSON object only (no markdown, no prose) with shape: " +
		`{"intent":"...","confidence":0-1,"language":"...","tool_calls":[{"name":"...","args":{}}],"assumptions":[],"unsupported_reason":null}` +
		"\nRules:\n" +
		"- Use ONLY intents from the tool intents= lists (sales, stock, advertising, alerts, priorities, recommendations, pricing, abc_analysis, general_overview, unknown, unsupported).\n" +
		"- For cabinet metric questions (revenue, orders, funnel, stock, ads, actions) ALWAYS include at least one matching tool_call whose intents list includes the chosen intent.\n" +
		"- For pure conversational/advice questions with no cabinet data need, use intent=general_overview and tool_calls=[].\n" +
		"- When Action context is provided and already answers the question, you may use tool_calls=[] with a domain intent only if tools would be redundant; prefer still calling a confirming tool.\n" +
		"- Never invent tool names or intents outside the allowlist.\n" +
		"- Prefer the smallest useful tool set (1-3 tools).\n" +
		"\n" + strings.Join(blocks, "\n")
}

func BuildPlannerUserPrompt(question string, asOfDate *time.Time, actionContext *ActionPromptContext) string {
	actionLine := formatActionPromptContext(actionContext)
	if asOfDate != nil {
		return fmt.Sprintf(
			"Question: %s\nAs of date: %s%s",
			question,
			asOfDate.UTC().Format("2006-01-02"),
			actionLine,
		)
	}
	end := time.Now().UTC().Truncate(24 * time.Hour)
	start := end.AddDate(0, 0, -DefaultChatDateRangeDays)
	return fmt.Sprintf(
		"Question: %s\nDefault analytics window: %s to %s (inclusive). As-of date: %s. The user may specify a different period in the question text.%s",
		question,
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
		end.Format("2006-01-02"),
		actionLine,
	)
}

func formatActionPromptContext(ctx *ActionPromptContext) string {
	if ctx == nil {
		return ""
	}
	if strings.TrimSpace(ctx.Title) == "" && strings.TrimSpace(ctx.ActionKey) == "" {
		return ""
	}
	lines := []string{"", "Action context from Actions Engine:"}
	lines = append(lines, "Title: "+ctx.Title)
	if ctx.Category != "" || ctx.Type != "" || ctx.Priority != "" {
		lines = append(lines, fmt.Sprintf("Category: %s; Type: %s; Priority: %s", ctx.Category, ctx.Type, ctx.Priority))
	}
	if strings.TrimSpace(ctx.Status) != "" {
		lines = append(lines, "Status: "+ctx.Status)
	}
	if strings.TrimSpace(ctx.MetricDate) != "" {
		lines = append(lines, "Analysis date: "+ctx.MetricDate)
	}
	if strings.TrimSpace(ctx.Reason) != "" {
		lines = append(lines, "Why: "+ctx.Reason)
	}
	if strings.TrimSpace(ctx.NextStep) != "" {
		lines = append(lines, "Next step: "+ctx.NextStep)
	}
	if strings.TrimSpace(ctx.ExpectedEffect) != "" {
		lines = append(lines, "Expected effect: "+ctx.ExpectedEffect)
	}
	if len(ctx.Evidence) > 0 {
		lines = append(lines, "Evidence:")
		for _, v := range ctx.Evidence {
			lines = append(lines, "- "+v)
		}
	}
	if len(ctx.EvidenceFactIDs) > 0 {
		lines = append(lines, "Evidence fact ids: "+strings.Join(ctx.EvidenceFactIDs, ", "))
	}
	if len(ctx.ObjectActions) > 0 {
		lines = append(lines, "Object actions:")
		for _, v := range ctx.ObjectActions {
			lines = append(lines, "- "+v)
		}
	}
	if strings.TrimSpace(ctx.CompletionCriteria) != "" {
		lines = append(lines, "Completion criteria: "+ctx.CompletionCriteria)
	}
	if len(ctx.Limitations) > 0 {
		lines = append(lines, "Limitations:")
		for _, v := range ctx.Limitations {
			lines = append(lines, "- "+v)
		}
	}
	return "\n" + strings.Join(lines, "\n")
}

func BuildAnswerSystemPrompt() string {
	return "You are an assistant. Use provided fact context and attached file contents only. Do not claim auto-actions. " +
		"When attached file contents are present, answer questions about those files using the extracted text and quote control markers exactly when asked. " +
		"For attachment-based answers use supporting_facts.source=\"attachment\". " +
		"When action_context is present, cite facts from it with supporting_facts.source=\"action_context\" and id=null. " +
		"Do not reference alert or recommendation ids unless they appear in the fact context. " +
		"Return STRICT JSON object only (no markdown, no prose) with shape: " +
		`{"answer":"...","summary":"...","intent":"...","confidence_level":"low|medium|high","related_alert_ids":[],"related_recommendation_ids":[],"supporting_facts":[{"source":"...","id":null,"fact":"..."}],"limitations":[]}`
}

// BuildStreamingAnswerSystemPrompt deliberately requests plain text: JSON
// formatting delays visible output and is handled locally after streaming.
func BuildStreamingAnswerSystemPrompt() string {
	return "You are a marketplace assistant. Answer the user in clear Russian or English, matching the question language. " +
		"Use only the provided fact context and attached file contents. Do not claim actions were performed, do not expose internal IDs unless present in context, and state material limitations. " +
		"Return only the user-facing answer as plain text; do not return JSON, markdown code fences, or meta commentary."
}

func BuildAnswerUserPrompt(ctx FactContext) string {
	raw, err := json.Marshal(ctx)
	if err != nil {
		return fmt.Sprintf("Question: %s\nIntent: %s\nTool results count: %d", ctx.Question, ctx.Intent, len(ctx.ToolResults))
	}
	out := fmt.Sprintf("Fact context JSON:\n%s", string(raw))
	if ctx.Action != nil {
		out += "\n\nAction-scoped question: answer using action_context facts first. " +
			"Use supporting_facts.source=\"action_context\" for evidence from action_context. " +
			"Keep related_alert_ids and related_recommendation_ids empty unless those ids are explicitly present in the fact context. " +
			"Copy visible action limitations into answer.limitations."
	}
	return out
}

func joinIntents(intents []ChatIntent) string {
	if len(intents) == 0 {
		return ""
	}
	out := make([]string, 0, len(intents))
	for _, intent := range intents {
		out = append(out, string(intent))
	}
	return strings.Join(out, "|")
}
