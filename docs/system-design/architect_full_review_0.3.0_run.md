P0 — пробелы, которые реально могут “порвать протокол” при интеграциях
1) Нет явного контракта “Session/Run” как первичного объекта

Где видно:

EVIDRA_CORE_DATA_MODEL.md: у Prescription есть trace_id (“Automation task/session correlation key”), но нет отдельного Session/Run объекта и нет чёткой семантики: это одна операция или вся серия операций?

EVIDRA_END_TO_END_EXAMPLE_v2.md: сценарии идут как “flow”, но не фиксируют границы сессии (start/end), нет общих полей для связывания серии операций.

EVIDRA_SIGNAL_SPEC.md: метрики есть, но не определено, что такое “run” с точки зрения подсчёта.

Почему это опасно:

LangChain/LangGraph/AutoGen/CrewAI естественно работают как “run” с множеством tool calls / steps.
Если у вас “trace_id” генерится на каждый prescribe, то:

scorecard на “агента” превращается в набор несвязанных операций

ретраи/циклы/поведение агента трудно мерить “в рамках одного run”

централизованный backend/реплей потом станет болью

Что фиксировать в дизайне (минимум для v1):

Ввести явный объект Session (Run): session_id (ULID) + started_at/ended_at.

У каждого события/операции всегда есть:

session_id (run boundary)

operation_id (или prescription_id как operation id)

trace_id/span_id — опционально как интеграция с distributed tracing.

В MCP контракт (prescribe) добавить:

session_id (MAY, если агент сам ведёт run)

run_name / labels (MAY)

или правило: “если не передано — Evidra генерит и возвращает, клиент обязан кэшировать и передавать дальше”.

Коротко: trace_id не должен быть единственным “run id”. Trace — это про трассировку, run — это про жизненный цикл автоматики.

2) “Scope / Environment” сейчас одномерен и может стать источником рассинхрона

Где видно:

CANONICALIZATION_CONTRACT_V1.md §10 “Scope Class Resolution”
scope_class строится из request.environment или namespace по frozen mapping (prefix match).

EVIDRA_CORE_DATA_MODEL.md: environment (MAY) в prescribe tool input.

EVIDRA_SIGNAL_SPEC.md: сигнал new_scope и метрики по scope.

Риск при будущих интеграциях:

Для Terraform namespace отсутствует, а environment часто не задан — scope будет “unknown”.

Для k8s “namespace=prod” не всегда означает production (мультикластеры, shared namespaces, preview environments).

Для cloud/infra часто важны account_id/region/cluster/workspace — одномерный scope_class не хватает.

Frozen mapping + prefix-match = потенциально непреднамеренные классификации (например prod-sandbox).

Что делать (не ломая вашу идею “scope_class frozen”):

Сохранить scope_class как низкокардинальный класс, но добавить Scope Dimensions (НЕ как labels в метриках, а как данные evidence):

scope: { environment, namespace, cluster, account, region, workspace, repo, branch } (все MAY)

В canonical action в digest продолжать включать только scope_class, а scope.dimensions хранить в EvidenceEntry/Prescription как метаданные (или отдельным полем), чтобы:

не ломать intent_digest от “шумных” деталей

иметь основу для “scope-aware compare” и будущей аналитики

И главное: правило приоритетов для Terraform и других:

terraform: workspace/account → derive scope_class

k8s: context/cluster + namespace → derive

generic: только environment, иначе unknown

И добавьте в контракт явную политику overrides (в CANONICALIZATION_CONTRACT_V1.md это уже упоминается рядом — scope_class_overrides), но нужно дописать:

как версионируются overrides

как обеспечить детерминизм при разных конфигурациях (иначе сравнение “между инстансами” сломается)

3) Нет чёткого протокола ingest для внешних валидаторов (findings)

Где видно:

EVIDRA_CORE_DATA_MODEL.md §4 описывает ValidatorFinding, но нет контракта “как это попадает в evidence”.

EVIDRA_END_TO_END_EXAMPLE_v2.md не показывает шаг “scanner -> findings”.

Код/архитектура подразумевают SARIF (есть отдельные упоминания в README и парсер), но доки в system-design не фиксируют API.

Почему это опасно:

В реальности findings приходят:

до prescribe (scan PR)

между prescribe и report (CI)

после report (post-deploy scan)

Если не зафиксировать протокол сейчас, интеграции сделают “как удобно”, и у вас появятся дыры связывания.

Нужно зафиксировать минимум:

Ingest endpoint/tool:

ingest_findings (MCP tool) или /v1/findings (для sidecar/REST)

Payload должен поддерживать:

SARIF (как первичный формат) + нормализованный минимальный формат

поля: tool, tool_version, run_id (scan run), artifact_digest, timestamp, rule_id, severity, resource, message

Семантика dedup:

dedup key = (artifact_digest, tool, rule_id, resource) + optional fingerprint

Правило “что делать при drift”:

если findings пришли на digest, который не совпадает с applied digest → явный статус “stale/other_artifact”

P1 — важные нестыковки/техдолг в логике, которые аукнутся при росте
4) “Trace/Correlation” не нормирован под multi-step агентные фреймворки

Где видно:

EVIDRA_CORE_DATA_MODEL.md: trace_id MUST в Prescription (stored), но в MCP input его нет — он “вычисляется”.

EVIDRA_END_TO_END_EXAMPLE_v2.md оперирует prescribe/report на шаги, но нет span-модели.

Что будет при LangGraph/AutoGen:

Нужна иерархия:

run/session

step/span (node/tool call)

parent-child

Решение:

Ввести простую span-модель как опциональные поля:

span_id, parent_span_id (строки)

trace_id (если хотят интеграцию с OpenTelemetry)

В метриках не использовать эти значения (кардинальность), но в evidence хранить.

5) Разные “planes” описаны, но границы интеграций пока не доведены до контракта

Где видно:

EVIDRA_SIGNAL_SPEC.md: “Evidence plane boundaries (MUST NOT cross)” — отлично.

EVIDRA_ARCHITECTURE_OVERVIEW.md: “Fourth Component: Signal Export” — но не описано:

кто экспортит /metrics (CLI? daemon? api?)

в каких режимах (local/service)

какие гарантии по “срезу” (на лету? по scorecard? периодически?)

Рекомендация:

В одном месте (лучше EVIDRA_SIGNAL_SPEC.md) зафиксировать:

“metrics exporter lives in evidra-agent/evidra-api”

“signals computed on-demand at scorecard time vs streaming”

если streaming — какие именно сигналы можно стримить без пересчёта

6) Actor/agent identity: риск взрывной кардинальности и путаницы “кто” vs “инстанс”

Где видно:

EVIDRA_CORE_DATA_MODEL.md: actor.id MUST (stable identifier)

EVIDRA_SIGNAL_SPEC.md: label agent=actor.id SHOULD be <50 unique values

Проблема в реальных интеграциях:

Многие будут делать actor.id = run_id или hostname → кардинальность улетит.

Фикс в протоколе:

Явно разделить:

actor.id = стабильный идентификатор (service account / bot name)

actor.instance_id (MAY) = под/контейнер/раннер (НЕ в метриках)

actor.version (MAY) = версия агента (можно в evidence, осторожно в метриках)

P2 — “шероховатости” в документации, которые создают расхождения в реализациях
7) Где и как версионируются контракты — недостаточно “операционно”

Где видно:

EVIDRA_SIGNAL_SPEC.md хорошо говорит про breaking change.

Но не зафиксировано единообразно:

какие поля MUST присутствовать в каждом EvidenceEntry (spec/canon/scoring version)

как читатель должен вести себя при mixed versions внутри одной chain

Нужно:

“Normative MUST list” для EvidenceEntry:

spec_version, canon_version, scoring_version

writer (mcp|cli|agent|api) + writer_version

Правило: chain может содержать разные версии, но scorecard должен уметь:

либо отказать с понятной ошибкой

либо деградировать (и задокументировать как)

Конкретно про “scope окружения” и “сессии” — что бы я зафиксировал как “нормативный протокол v1”
Нормативные поля, которые должны появиться во всех будущих входах (MCP/REST/CLI)

MUST

session_id (run boundary)

operation_id (или prescription_id)

timestamp

SHOULD

trace_id, span_id, parent_span_id (для графов/агентов)

scope_class (derived) + scope.dimensions (raw)

actor.id (stable), actor.type, actor.provenance

MAY

labels (низкокардинальные: repo, env, pipeline)

Резюме: что сейчас самое “опасное” по пробелам

Нет “session/run” как первичного объекта → интеграции будут делать по-своему.

Scope одномерен и k8s-центричен → Terraform/Cloud/мультикластеры дадут “unknown” и сломают сравнение/сигналы.

Validator findings есть как модель, но нет ingest-контракта → неизбежно появится хаос.