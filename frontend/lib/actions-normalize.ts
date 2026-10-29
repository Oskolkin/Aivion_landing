import type {
  ActionAffectedEntities,
  ActionCTA,
  ActionCompletionCondition,
  ActionCounters,
  ActionEvidence,
  ActionItem,
  ActionsAIFocus,
  ActionsResponse,
  ActionsResponseStatus,
  ActionObjectActionsCoverage,
  OperationalDetail,
} from "@/lib/actions-types";

function asString(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function asStringArray(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) return undefined;
  return value.filter((item): item is string => typeof item === "string");
}

function asRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === "object") return value as Record<string, unknown>;
  return {};
}

function normalizeEvidence(raw: unknown): ActionEvidence[] {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => {
    const record = asRecord(item);
    return {
      kind: asString(record.kind),
      label: asString(record.label),
      value: asString(record.value),
      unit: asString(record.unit) || undefined,
      direction: asString(record.direction) || undefined,
      description: asString(record.description) || undefined,
    };
  });
}

function normalizeCTAs(raw: unknown): ActionCTA[] {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => {
    const record = asRecord(item);
    const paramsRaw = asRecord(record.params);
    const params: Record<string, string> = {};
    for (const [key, value] of Object.entries(paramsRaw)) {
      if (typeof value === "string") params[key] = value;
    }
    return {
      type: asString(record.type),
      label: asString(record.label),
      href: asString(record.href) || undefined,
      export_type: asString(record.export_type) || undefined,
      prompt: asString(record.prompt) || undefined,
      params: Object.keys(params).length ? params : undefined,
    };
  });
}

function normalizeAffectedEntities(raw: unknown): ActionAffectedEntities | undefined {
  const record = asRecord(raw);
  if (!Object.keys(record).length) return undefined;
  const skus = Array.isArray(record.skus)
    ? record.skus.map((item) => {
        const sku = asRecord(item);
        return {
          sku: asString(sku.sku),
          offer_id: asString(sku.offer_id) || undefined,
          name: asString(sku.name) || undefined,
          image_url: asString(sku.image_url) || undefined,
        };
      })
    : undefined;
  const campaigns = Array.isArray(record.campaigns)
    ? record.campaigns.map((item) => {
        const campaign = asRecord(item);
        return {
          campaign_id: asString(campaign.campaign_id),
          name: asString(campaign.name) || undefined,
        };
      })
    : undefined;
  const funnel = record.funnel ? asRecord(record.funnel) : null;
  return {
    skus,
    campaigns,
    funnel: funnel
      ? {
          transition_key: asString(funnel.transition_key),
          from_stage: asString(funnel.from_stage),
          to_stage: asString(funnel.to_stage),
        }
      : null,
  };
}

function normalizeAIFocus(raw: unknown): ActionsAIFocus | undefined {
  if (!raw) return undefined;
  const record = asRecord(raw);
  return {
    status: asString(record.status),
    source: asString(record.source),
    headline: asString(record.headline),
    summary: asString(record.summary),
    top_categories: asStringArray(record.top_categories),
    limitations: asStringArray(record.limitations),
    start_with: asString(record.start_with) || undefined,
    priority_order: asStringArray(record.priority_order),
  };
}

function normalizeCounters(raw: unknown): ActionCounters {
  const record = asRecord(raw);
  const toNumber = (value: unknown) => (typeof value === "number" ? value : 0);
  return {
    total_open: toNumber(record.total_open),
    critical: toNumber(record.critical),
    important: toNumber(record.important),
    watch: toNumber(record.watch),
    stock: toNumber(record.stock),
    sku: toNumber(record.sku),
    advertising: toNumber(record.advertising),
    funnel: toNumber(record.funnel),
    sales: toNumber(record.sales),
    checked: toNumber(record.checked),
    slow_turnover: toNumber(record.slow_turnover),
    category_count: toNumber(record.category_count),
    dismissed: toNumber(record.dismissed),
  };
}

function normalizeOperationalDetail(raw: unknown): OperationalDetail | undefined {
  if (!raw) return undefined;
  const record = asRecord(raw);
  if (!record.format_version && !record.why && !record.action_plan) return undefined;
  const why = asRecord(record.why);
  const plan = asRecord(record.action_plan);
  const stepsRaw = Array.isArray(plan.steps) ? plan.steps : [];
  const factsRaw = Array.isArray(why.facts) ? why.facts : [];
  const objectActionsRaw = Array.isArray(record.object_actions) ? record.object_actions : [];
  const secondaryRaw = Array.isArray(record.secondary_actions) ? record.secondary_actions : [];
  return {
    status: asString(record.status, "available"),
    format_version: asString(record.format_version, "actions_detail_v2"),
    why: {
      headline: asString(why.headline),
      text: asString(why.text),
      facts: factsRaw.map((item) => {
        const fact = asRecord(item);
        return {
          fact_id: asString(fact.fact_id),
          label: asString(fact.label),
          value: asString(fact.value),
          unit: asString(fact.unit) || undefined,
          formatted_value: asString(fact.formatted_value) || undefined,
        };
      }),
    },
    action_plan: {
      headline: asString(plan.headline, "Что сделать"),
      steps: stepsRaw.map((item) => {
        const step = asRecord(item);
        const targets = Array.isArray(step.target_entities)
          ? step.target_entities.map((ent) => {
              const e = asRecord(ent);
              return {
                entity_type: asString(e.entity_type),
                id: asString(e.id),
                name: asString(e.name) || undefined,
              };
            })
          : undefined;
        return {
          step_id: asString(step.step_id),
          action_kind: asString(step.action_kind) || undefined,
          title: asString(step.title),
          instruction: asString(step.instruction),
          why_this_step: asString(step.why_this_step),
          expected_result: asString(step.expected_result),
          confidence: asString(step.confidence) || undefined,
          decision_mode: asString(step.decision_mode) || undefined,
          evidence_fact_ids: asStringArray(step.evidence_fact_ids),
          target_entities: targets,
        };
      }),
    },
    object_actions: objectActionsRaw.length
      ? objectActionsRaw.map((item) => {
          const obj = asRecord(item);
          const metrics = Array.isArray(obj.metrics)
            ? obj.metrics.map((m) => {
                const metric = asRecord(m);
                const displayLabel = asString(metric.display_label) || asString(metric.label);
                const formattedValue = asString(metric.formatted_value) || asString(metric.value);
                return {
                  label: displayLabel,
                  value: formattedValue,
                  metric_code: asString(metric.metric_code) || undefined,
                  display_label: displayLabel || undefined,
                  formatted_value: formattedValue || undefined,
                };
              })
            : undefined;
          return {
            entity_type: asString(obj.entity_type),
            entity_id: asString(obj.entity_id),
            name: asString(obj.name) || undefined,
            metrics,
            recommended_action: asString(obj.recommended_action),
            reason: asString(obj.reason) || undefined,
            confidence: asString(obj.confidence) || undefined,
          };
        })
      : undefined,
    secondary_actions: secondaryRaw.length
      ? secondaryRaw.map((item) => {
          const sec = asRecord(item);
          return { type: asString(sec.type), label: asString(sec.label) };
        })
      : undefined,
    hidden_limitations: asStringArray(record.hidden_limitations),
  };
}

function normalizeObjectActionsCoverage(raw: unknown): ActionObjectActionsCoverage | undefined {
  if (!raw) return undefined;
  const record = asRecord(raw);
  const mode = asString(record.mode);
  if (!mode) return undefined;
  return {
    mode,
    reason: asString(record.reason) || undefined,
  };
}

function normalizeCompletionConditions(raw: unknown): ActionCompletionCondition[] | undefined {
  if (!Array.isArray(raw)) return undefined;
  return raw.map((item) => {
    const record = asRecord(item);
    return {
      metric_code: asString(record.metric_code),
      operator: asString(record.operator),
      target_value: typeof record.target_value === "number" ? record.target_value : undefined,
      target_value_max: typeof record.target_value_max === "number" ? record.target_value_max : undefined,
      fact_id: asString(record.fact_id) || undefined,
      scope: asString(record.scope),
      period_days: typeof record.period_days === "number" ? record.period_days : undefined,
      display_text: asString(record.display_text) || undefined,
    };
  });
}

export function normalizeActionItem(raw: unknown): ActionItem {
  const record = asRecord(raw);
  return {
    id: typeof record.id === "number" ? record.id : 0,
    action_key: asString(record.action_key),
    type: asString(record.type) as ActionItem["type"],
    category: asString(record.category) as ActionItem["category"],
    priority: asString(record.priority) as ActionItem["priority"],
    execution_tier: asString(record.execution_tier) || undefined,
    status: asString(record.status) as ActionItem["status"],
    title: asString(record.title),
    summary: asString(record.summary) || undefined,
    reason: asString(record.reason),
    next_step: asString(record.next_step),
    expected_effect: asString(record.expected_effect) || undefined,
    evidence: normalizeEvidence(record.evidence),
    limitations: asStringArray(record.limitations),
    ctas: normalizeCTAs(record.ctas),
    affected_entities: normalizeAffectedEntities(record.affected_entities),
    source_candidate_ids: asStringArray(record.source_candidate_ids),
    evidence_fact_ids: asStringArray(record.evidence_fact_ids),
    entity_group_ref: asString(record.entity_group_ref) || undefined,
    operational_detail: normalizeOperationalDetail(record.operational_detail),
    urgency_marker: asString(record.urgency_marker) || undefined,
    affected_objects_summary: asString(record.affected_objects_summary) || undefined,
    affected_objects_total: typeof record.affected_objects_total === "number" ? record.affected_objects_total : undefined,
    affected_objects_displayed:
      typeof record.affected_objects_displayed === "number" ? record.affected_objects_displayed : undefined,
    affected_objects_truncated:
      typeof record.affected_objects_truncated === "boolean" ? record.affected_objects_truncated : undefined,
    operational_goal: asString(record.operational_goal) || undefined,
    scope_type: asString(record.scope_type) as ActionItem["scope_type"],
    object_actions_coverage: normalizeObjectActionsCoverage(record.object_actions_coverage),
    completion_criteria: asString(record.completion_criteria) || undefined,
    completion_conditions: normalizeCompletionConditions(record.completion_conditions),
    visible_limitations: asStringArray(record.visible_limitations),
    metric_date: asString(record.metric_date) || undefined,
    created_at: asString(record.created_at) || undefined,
    updated_at: asString(record.updated_at) || undefined,
    checked_at: asString(record.checked_at) || undefined,
  };
}

export function normalizeActionsResponse(raw: unknown): ActionsResponse {
  const record = asRecord(raw);
  const actionsRaw = Array.isArray(record.actions) ? record.actions : [];
  const status = asString(record.status, "empty") as ActionsResponseStatus;
  const publicationId =
    typeof record.publication_id === "number" ? record.publication_id : undefined;
  const pipelineRunId =
    typeof record.pipeline_run_id === "number" ? record.pipeline_run_id : undefined;
  return {
    metric_date: asString(record.metric_date) || undefined,
    status,
    ai_focus: normalizeAIFocus(record.ai_focus),
    counters: normalizeCounters(record.counters),
    actions: actionsRaw.map((item) => normalizeActionItem(item)),
    data_quality_notes: asStringArray(record.data_quality_notes),
    analysis_source: asString(record.analysis_source) || undefined,
    publication_id: publicationId,
    pipeline_run_id: pipelineRunId,
    fallback_used: record.fallback_used === true,
    fallback_reason: asString(record.fallback_reason) || undefined,
  };
}
