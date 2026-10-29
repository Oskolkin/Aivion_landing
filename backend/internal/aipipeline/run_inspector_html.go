package aipipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
)

const inspectorHTMLTemplate = `<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="utf-8"/>
<title>AI Pipeline Inspector — Run {{index .summary "pipeline_run_id"}}</title>
<style>
body{font-family:system-ui,-apple-system,Segoe UI,sans-serif;margin:0;background:#0f1419;color:#e6edf3}
header{padding:16px 24px;background:#161b22;border-bottom:1px solid #30363d}
h1{font-size:20px;margin:0 0 8px}
.meta{display:flex;flex-wrap:wrap;gap:12px;font-size:13px;color:#8b949e}
nav{display:flex;gap:8px;padding:12px 24px;background:#161b22;border-bottom:1px solid #30363d;flex-wrap:wrap}
nav button{background:#21262d;border:1px solid #30363d;color:#e6edf3;padding:6px 10px;border-radius:6px;cursor:pointer;font-size:12px}
nav button.active{background:#388bfd;border-color:#388bfd;color:#fff}
main{padding:20px 24px}
.panel{display:none}.panel.active{display:block}
table{border-collapse:collapse;width:100%;font-size:13px;margin-bottom:16px}
th,td{border:1px solid #30363d;padding:8px;text-align:left;vertical-align:top}
th{background:#161b22}
pre{background:#0d1117;border:1px solid #30363d;padding:12px;border-radius:8px;overflow:auto;max-height:480px;font-size:12px}
.notice{color:#d29922;font-size:13px}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(240px,1fr));gap:12px}
.card{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:12px}
.card h3{margin:0 0 8px;font-size:14px}
.search{margin:8px 0 16px;padding:8px 12px;width:100%;max-width:480px;background:#0d1117;border:1px solid #30363d;color:#e6edf3;border-radius:6px}
details{margin-top:12px} summary{cursor:pointer;color:#8b949e}
.highlight{background:#388bfd33}
.filter-bar{display:flex;gap:8px;flex-wrap:wrap;margin:8px 0 16px}
.filter-bar button{background:#21262d;border:1px solid #30363d;color:#e6edf3;padding:6px 10px;border-radius:6px;cursor:pointer;font-size:12px}
</style>
</head>
<body>
<header>
<h1>AI Pipeline Inspector</h1>
<div class="meta">
<span>Run ID: {{index .summary "pipeline_run_id"}}</span>
<span>Status: {{index .summary "status"}}</span>
<span>Metric date: {{index .summary "metric_date"}}</span>
<span>Seller: {{index .summary "seller_account_id"}}</span>
<span>Actions adapter: {{index .summary "actions_adapter"}}</span>
</div>
</header>
<nav id="nav"></nav>
<main id="main"></main>
<script>
const DATA = {{json .}};
const PANELS = [
  {id:'overview', label:'Overview'},
  {id:'diagnostics', label:'Diagnostics'},
  {id:'ai_calls', label:'AI Calls'},
  {id:'stages', label:'Stages'},
  {id:'actions_input', label:'Actions Input'},
  {id:'data_pack', label:'Data Pack'},
  {id:'chunks', label:'Chunks'},
  {id:'signals', label:'Signals'},
  {id:'candidates', label:'Candidates'},
  {id:'facts', label:'Facts'},
  {id:'entities', label:'Entities'},
  {id:'global_context', label:'Global Context'},
  {id:'actions_compose', label:'Action Compose'},
  {id:'legacy_actions', label:'Legacy Actions'},
  {id:'validation', label:'Validation'},
  {id:'normalization', label:'Normalization'},
  {id:'object_details', label:'Object Details'},
  {id:'final_actions', label:'Final Actions'},
  {id:'lineage', label:'Lineage'},
  {id:'metric_semantics', label:'Metric Semantics'},
  {id:'chunk_ref_mappings', label:'Chunk Ref Mappings'},
  {id:'gc_validation', label:'GC Validation'},
  {id:'final_validation', label:'Final Validation'},
  {id:'actions_execution', label:'Actions Execution'},
  {id:'publication_guard', label:'Publication Guard'},
  {id:'artifacts', label:'Artifacts'}
];
function esc(s){return String(s??'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');}
function link(kind,id,label){if(!id)return esc(label??'');const safe=String(id).replace(/[^a-zA-Z0-9:_\-]/g,'-');return '<a class="link" href="#'+kind+'-'+safe+'">'+esc(label??id)+'</a>';}
function jsonBlock(obj){return '<details><summary>Show raw JSON</summary><pre>'+esc(JSON.stringify(obj,null,2))+'</pre></details>';}
function tableFromRows(headers, rows, rowAttrs){
  if(!rows||!rows.length) return '<p class="notice">No data</p>';
  let h='<table><thead><tr>'+headers.map(x=>'<th>'+esc(x)+'</th>').join('')+'</tr></thead><tbody>';
  rows.forEach((r,i)=>{const attrs=rowAttrs&&rowAttrs[i]?(' '+rowAttrs[i]):''; h+='<tr'+attrs+'>'+r.map(c=>'<td>'+c+'</td>').join('')+'</tr>';});
  return h+'</tbody></table>';
}
function renderOverview(){
  const diag=(DATA.views&&DATA.views.diagnostics)||{};
  const statuses=diag.statuses||{};
  const cards=[
    ['Run status', DATA.summary&&DATA.summary.status],
    ['Published', DATA.summary&&DATA.summary.published],
    ['Actions adapter', DATA.summary&&DATA.summary.actions_adapter],
    ['Source run', DATA.summary&&DATA.summary.source_pipeline_run_id],
    ['Raw body available', statuses.raw_body_available],
    ['Validation outcome', statuses.validation_outcome],
    ['Actions stage', statuses.actions_stage_status],
    ['Pipeline status', statuses.pipeline_status],
  ];
  let html='<section id="overview" class="panel"><h2>Overview</h2><div class="grid">'
    +cards.map(([k,v])=>'<div class="card"><h3>'+esc(k)+'</h3><div>'+esc(v)+'</div></div>').join('')
    +'</div>';
  if(statuses.truncation_note){html+='<p class="notice">'+esc(statuses.truncation_note)+'</p>';}
  if(diag.failure_focus&&diag.failure_focus.applicable){
    html+='<h3>Failure Focus</h3><p><strong>Primary:</strong> '+esc(diag.failure_focus.primary_failure)+'</p>';
    if(diag.failure_focus.provider_returned_count!==undefined){html+='<p>Provider returned count: '+esc(diag.failure_focus.provider_returned_count)+' ('+esc(diag.failure_focus.provider_returned_status)+')</p>';}
    html+='<p>Validation processed: '+esc(diag.failure_focus.validation_processed_count)+'; dropped: '+esc(diag.failure_focus.dropped_count)+'; persisted: '+esc(diag.failure_focus.persisted_count)+'</p>';
    html+=jsonBlock(diag.failure_focus);
  }
  return html+'</section>';
}
function renderDiagnostics(){
  const diag=(DATA.views&&DATA.views.diagnostics)||{};
  const completeness=(diag.completeness||[]).map(e=>[esc(e.dimension),esc(e.status),esc(e.reason||''),esc(e.artifact_id||'')]);
  const timeline=(diag.timeline||[]).map(e=>[esc(e.event),esc(e.stage)]);
  return '<section id="diagnostics" class="panel"><h2>Diagnostic Completeness</h2>'
    +tableFromRows(['Diagnostic data','Status','Reason','Artifact ID'], completeness)
    +'<h3>Stage Timeline</h3>'+tableFromRows(['Event','Stage'], timeline)
    +jsonBlock(diag)+'</section>';
}
function renderAICalls(){
  const ai=(DATA.views&&DATA.views.diagnostics&&DATA.views.diagnostics.ai_calls)||{};
  const rows=(ai.summary||[]).map(r=>[
    esc(r.stage), esc(r.component), esc(r.execution), esc(r.ai_call_in_current_run),
    esc(r.source_pipeline_run_id??''), esc(r.ai_calls??''), esc(r.diagnostics??''), esc(r.raw_body_status??''), esc(r.parse_status??''), esc(r.stage_outcome??'')
  ]);
  return '<section id="ai_calls" class="panel"><h2>AI Calls</h2>'
    +tableFromRows(['Stage','Component','Execution','AI call (current run)','Source run','Calls','Diagnostics','Raw body','Parse','Outcome'], rows)
    +jsonBlock(ai)+'</section>';
}
function renderActionsInput(){
  const input=(DATA.views&&DATA.views.actions_input)||{};
  if(!input.available) return '<section id="actions_input" class="panel"><h2>Actions Input</h2><p class="notice">Actions input artifact not available</p></section>';
  const summary=[
    ['schema version', input.schema_version], ['candidate count', input.candidate_count], ['fact count', input.fact_count],
    ['entity count', input.entity_count], ['max actions', input.max_actions], ['failed chunks', input.failed_chunk_count],
    ['data pack hash', input.data_pack_hash], ['global context hash', input.global_context_hash],
  ];
  let html='<section id="actions_input" class="panel"><h2>Actions Input</h2><div class="grid">'
    +summary.map(([k,v])=>'<div class="card"><h3>'+esc(k)+'</h3><div>'+esc(v)+'</div></div>').join('')+'</div>';
  html+='<h3>Candidates</h3><div id="actions_input_candidates"><div class="filter-bar"><input class="search" placeholder="Search candidates..." data-filter="actions_input_candidates"/>'
    +'<button type="button" data-clear="actions_input_candidates">Clear filters</button></div>';
  const candRows=(input.candidates||[]).map(c=>[
    link('candidate', c.candidate_id, c.candidate_id), esc(c.chunk_id), esc(c.type), esc(c.category), esc(c.local_priority||c.priority), esc(c.problem)
  ]);
  const candAttrs=(input.candidates||[]).map(c=>'id="candidate-'+String(c.candidate_id||'').replace(/[^a-zA-Z0-9:_\-]/g,'-')+'" data-search="'+esc(c.search_text||'')+'"');
  html+=tableFromRows(['Candidate ID','Chunk','Type','Category','Priority','Problem'], candRows, candAttrs);
  html+='</div>'+jsonBlock(input.raw);
  return html+'</section>';
}
function renderStages(){
  const rows=(DATA.stages||[]).map(s=>[
    esc(s.stage), esc(s.execution||s.source), esc(s.source), esc(s.source_run??''), esc(s.ai_calls??''), esc(s.outcome||s.status)
  ]);
  return '<section id="stages" class="panel"><h2>Stages</h2>'+tableFromRows(['Stage','Execution','Source','Source run','AI calls','Outcome'], rows)+'</section>';
}
function renderListPanel(id,title,items,cols,mapRow){
  const mapped=(items||[]).map(mapRow);
  const rows=mapped.map(m=>m.cells||m);
  const attrs=mapped.map(m=>m.rowAttrs||'');
  return '<section id="'+id+'" class="panel"><h2>'+title+'</h2>'
    +'<div class="filter-bar"><input class="search" placeholder="Search..." data-filter="'+id+'"/>'
    +'<button type="button" data-clear="'+id+'">Clear filters</button></div>'
    +tableFromRows(cols, rows, attrs)+jsonBlock(items)+'</section>';
}
function renderSectionPanel(id,title,key){
  const data=(DATA.sections||{})[key];
  if(!data) return '<section id="'+id+'" class="panel"><h2>'+title+'</h2><p class="notice">Artifact not available for this pipeline version</p></section>';
  const notice=data.notice?('<p class="notice">'+esc(data.notice)+'</p>'):'';
  return '<section id="'+id+'" class="panel"><h2>'+title+'</h2>'+notice+jsonBlock(data)+'</section>';
}
function renderMonitoringPanel(id,title,key){
  const mon=(DATA.views&&DATA.views.monitoring)||{};
  const data=mon[key];
  if(!data) return '<section id="'+id+'" class="panel"><h2>'+title+'</h2><p class="notice">Artifact not available for this pipeline version</p></section>';
  const notice=data.status==='not_available'?('<p class="notice">'+esc(data.message||'Artifact not available for this pipeline version')+'</p>'):'';
  return '<section id="'+id+'" class="panel"><h2>'+title+'</h2>'+notice+jsonBlock(data)+'</section>';
}
function renderViews(){
  const v=DATA.views||{};
  let html=renderOverview()+renderDiagnostics()+renderAICalls()+renderStages()+renderActionsInput();
  html+=renderSectionPanel('data_pack','Data Pack','data_pack');
  html+=renderSectionPanel('chunks','Shared Chunks','shared_chunk_analysis');
  html+=renderListPanel('signals','Signals',v.signals,['Signal ID','Chunk','Chunk source','Type','Observation','Status'],s=>({
    cells:[link('signal', s.signal_id, s.signal_id), esc(s.chunk_id), esc(s.chunk_id_source), esc(s.type), esc(s.observation), esc(s.status)],
    rowAttrs:'id="signal-'+String(s.signal_id||'').replace(/[^a-zA-Z0-9:_\-]/g,'-')+'"'
  }));
  html+=renderListPanel('candidates','Candidates',v.candidates,['Candidate ID','Chunk','Type','Problem','Disposition'],c=>({
    cells:[link('candidate', c.candidate_id, c.candidate_id), esc(c.chunk_id), esc(c.type), esc(c.problem), esc(c.disposition||c.status)],
    rowAttrs:'id="candidate-'+String(c.candidate_id||'').replace(/[^a-zA-Z0-9:_\-]/g,'-')+'"'
  }));
  html+=renderListPanel('facts','Facts',v.facts,['Fact ID','Metric','Entity','Value','Unit'],f=>({
    cells:[link('fact', f.fact_id, f.fact_id), esc(f.metric_code), link('entity', f.entity_id, f.entity_id), esc(f.value), esc(f.unit)],
    rowAttrs:'id="fact-'+String(f.fact_id||'').replace(/[^a-zA-Z0-9:_\-]/g,'-')+'"'
  }));
  html+=renderListPanel('entities','Entities',v.entities,['Entity ID','Type','Name','Offer ID'],e=>({
    cells:[link('entity', e.entity_id, e.entity_id), esc(e.entity_type), esc(e.name), esc(e.offer_id)],
    rowAttrs:'id="entity-'+String(e.entity_id||'').replace(/[^a-zA-Z0-9:_\-]/g,'-')+'"'
  }));
  html+=renderSectionPanel('global_context','Global Context','global_context');
  html+=renderSectionPanel('actions_compose','Action Compose','actions_compose_output');
  if(v.legacy_actions){html+='<section id="legacy_actions" class="panel"><h2>Legacy Actions</h2>'+jsonBlock(v.legacy_actions)+'</section>';}
  else {html+='<section id="legacy_actions" class="panel"><h2>Legacy Actions</h2><p class="notice">Not a legacy run</p></section>';}
  if(v.validation){html+=renderValidationPanel(v.validation);}
  else {html+=renderSectionPanel('validation','Validation','actions_validation');}
  if(v.normalization){html+='<section id="normalization" class="panel"><h2>Normalization</h2>'+jsonBlock(v.normalization)+'</section>';}
  else {html+=renderSectionPanel('normalization','Normalization','actions_normalization');}
  html+=renderSectionPanel('object_details','Object Details','actions_object_detail_merge');
  html+=renderSectionPanel('final_actions','Final Actions','actions_final_output');
  html+=renderSectionPanel('lineage','Lineage','ai_pipeline_lineage');
  html+=renderMonitoringPanel('metric_semantics','Metric Semantics','metric_semantics');
  html+=renderMonitoringPanel('chunk_ref_mappings','Chunk Ref Mappings','chunk_ref_mappings');
  html+=renderMonitoringPanel('gc_validation','Global Context Validation','global_context_validation');
  html+=renderMonitoringPanel('final_validation','Final Output Validation','final_output_validation');
  html+=renderMonitoringPanel('actions_execution','Actions Execution','actions_execution');
  html+=renderMonitoringPanel('publication_guard','Publication Guard','publication_guard');
  html+=renderListPanel('artifacts','Artifacts',v.artifacts,['ID','Kind','Schema','Content','Raw body','Adapter'],a=>[
    esc(a.id), esc(a.kind), esc(a.schema_version), esc(a.content_completeness||a.parsed_status), esc(a.raw_body_available), esc(a.adapter_used)
  ]);
  return html;
}
function renderValidationPanel(v){
  const issues=[].concat(v.normalization||[], v.warnings||[], v.blocking_errors||[], v.persist_guard||[]);
  const rows=issues.map(issue=>{
    const code=issue.code||issue.event_code||'issue';
    return {cells:[link('issue', code, code), esc(issue.severity), esc(issue.message||issue.details&&issue.details.message||'')], rowAttrs:'id="issue-'+String(code).replace(/[^a-zA-Z0-9:_\-]/g,'-')+'"'};
  });
  let html='<section id="validation" class="panel"><h2>Validation</h2>';
  if(v.drop_reasons&&v.drop_reasons.length){html+='<h3>Drop reasons</h3>'+jsonBlock(v.drop_reasons);}
  html+=tableFromRows(['Issue','Severity','Message'], rows.map(r=>r.cells), rows.map(r=>r.rowAttrs));
  html+=jsonBlock(v);
  return html+'</section>';
}
function filterContainer(containerId, query){
  const container=document.getElementById(containerId);
  if(!container) return;
  const q=String(query||'').toLowerCase();
  container.querySelectorAll('tbody tr').forEach(row=>{
    const text=(row.dataset.search||row.textContent||'').toLowerCase();
    row.style.display = !q || text.includes(q) ? '' : 'none';
  });
}
function filterTable(panelId, query){
  filterContainer(panelId, query);
}
function wireSearch(){
  document.querySelectorAll('input.search').forEach(input=>{
    input.addEventListener('input', ()=>filterContainer(input.dataset.filter, input.value));
  });
  document.querySelectorAll('button[data-clear]').forEach(btn=>{
    btn.addEventListener('click', ()=>{
      const target=btn.dataset.clear;
      const input=document.querySelector('input.search[data-filter="'+target+'"]');
      if(input){ input.value=''; filterContainer(target, ''); }
    });
  });
}
const LINEAGE_PREFIX_TO_PANEL={signal:'signals',candidate:'candidates',fact:'facts',entity:'entities',action:'legacy_actions',issue:'validation'};
function panelForHash(hash){
  if(!hash) return null;
  if(document.getElementById(hash)){
    const prefix=hash.split('-')[0];
    if(LINEAGE_PREFIX_TO_PANEL[prefix]) return LINEAGE_PREFIX_TO_PANEL[prefix];
  }
  if(PANELS.some(p=>p.id===hash)) return hash;
  return null;
}
function followHash(hash){
  const clean=String(hash||'').replace(/^#/,'');
  if(!clean) return;
  const panel=panelForHash(clean);
  if(panel) activatePanel(panel, {keepHash:true});
  const el=document.getElementById(clean);
  if(el){
    document.querySelectorAll('.highlight').forEach(n=>n.classList.remove('highlight'));
    el.classList.add('highlight');
    el.scrollIntoView({block:'center'});
  }
}
function activatePanel(panelId, opts={}){
  document.querySelectorAll('nav button').forEach(b=>b.classList.toggle('active',b.dataset.panel===panelId));
  document.querySelectorAll('.panel').forEach(p=>p.classList.toggle('active',p.id===panelId));
  if(!opts.keepHash) location.hash=panelId;
}
function init(){
  const nav=document.getElementById('nav');
  PANELS.forEach(p=>{
    const btn=document.createElement('button');
    btn.textContent=p.label; btn.dataset.panel=p.id;
    btn.addEventListener('click',()=>activatePanel(p.id));
    nav.appendChild(btn);
  });
  document.getElementById('main').innerHTML=renderViews();
  wireSearch();
  document.addEventListener('click', e=>{
    const a=e.target.closest('a.link');
    if(!a) return;
    const href=a.getAttribute('href')||'';
    if(!href.startsWith('#')) return;
    e.preventDefault();
    location.hash=href.slice(1);
  });
  window.addEventListener('hashchange', ()=>followHash(location.hash));
  const initial=(location.hash||'').replace('#','');
  const startPanel=panelForHash(initial)|| (PANELS.some(p=>p.id===initial)?initial:'overview');
  activatePanel(startPanel, {keepHash:!!panelForHash(initial)});
  if(initial) followHash(initial);
}
init();
</script>
</body>
</html>`

func writeInspectorHTML(outputDir string, data map[string]any) error {
	funcMap := template.FuncMap{
		"json": func(v any) template.JS {
			raw, err := json.Marshal(v)
			if err != nil {
				return template.JS("null")
			}
			return template.JS(raw)
		},
		"dict": func(values ...any) map[string]any {
			m := map[string]any{}
			for i := 0; i+1 < len(values); i += 2 {
				m[fmt.Sprint(values[i])] = values[i+1]
			}
			return m
		},
	}
	tmpl, err := template.New("inspector").Funcs(funcMap).Parse(inspectorHTMLTemplate)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, "index.html"), buf.Bytes(), 0o644)
}
