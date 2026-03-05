e2e тесты тут как раз можно сделать так, чтобы они одновременно закрепили эти 3 задачи и стали основой для GitHub Action. Ниже — конкретный план e2e, структура тестов, сценарии и как увязать с Action.

Принцип e2e: “one run → evidence → validate → scorecard → compare”

Один e2e сценарий должен:

создать session/run

записать несколько событий (prescribe/report + findings до/после)

подписать chain

прогнать validate (hash + signatures)

получить scorecard per-session

(опционально) compare с golden

E2E тесты (минимальный набор)
E2E-1: Signing end-to-end

Цель: доказать, что подписи реально пишутся и валидируются.

Шаги:

создать evidence dir

evidra prescribe ... --session-id S1 ...

evidra report ... --session-id S1 ...

evidra validate --path evidence --signatures

негативный кейс: изменить 1 байт в segment_0001.jsonl → validate должен упасть

Проверки:

у каждой entry есть signature

validate возвращает success

после tamper: ошибка signature invalid или hash mismatch

E2E-2: Session-first scoring

Цель: scorecard агрегируется по session_id, а не “в среднем по всему”.

Шаги:

run A: session S1 (например безопасный сценарий)

run B: session S2 (специально добавить “bad behavior”: retry loop / blast radius / forbidden scope)

evidra scorecard --session-id S1 → ожидаем lower risk

evidra scorecard --session-id S2 → ожидаем higher risk

evidra scorecard --all-sessions → выдаёт 2 результата

Проверки:

вывод содержит session_id

risk levels различаются предсказуемо

golden output фиксирует стабильность

E2E-3: Findings ingestion до/после prescribe

Цель: findings можно добавить независимо, и они связываются с run.

Шаги (пример):

evidra session start → возвращает S1 (или просто фиксированный --session-id)

evidra ingest-findings --session-id S1 --artifact sha256:X --sarif tests/fixtures/trivy.sarif

evidra prescribe --session-id S1 --artifact sha256:X ...

evidra report --session-id S1 --artifact sha256:X ...

evidra ingest-findings --session-id S1 --artifact sha256:X --sarif tests/fixtures/kubescape.sarif

evidra scorecard --session-id S1 → score должен учитывать findings (например risk↑)

Проверки:

findings появляются в evidence как EntryTypeFinding (или отдельный тип)

связаны по session_id + artifact_digest

scorecard включает сигналы/summary “findings present”

Как это оформить в репозитории
Структура
tests/e2e/
  README.md
  fixtures/
    trivy.sarif
    kubescape.sarif
    terraform_plan.json (optional)
  golden/
    e2e_signing.json
    e2e_sessions.json
    e2e_findings.json
  scripts/
    run_e2e.sh
    tamper_segment.py
Важно

e2e тесты должны работать без интернета

не зависеть от реального terraform/k8s (только фикстуры)

использовать deterministic ids (или мок-режим)

Нужные CLI флаги/команды (минимально)

Чтобы e2e были устойчивыми, удобно иметь:

evidra validate --signatures

evidra scorecard --session-id <id> и/или --group-by session

evidra ingest-findings ... (CLI)

(опционально) evidra session start/end или просто явный --session-id везде

--clock-fixed или возможность передать timestamp (чтобы golden не плавал)

GitHub Action плагин: как связать всё вместе
Идея Action

Action делает 3 вещи:

собирает evidence во время CI

валидирует evidence (hash + signatures)

публикует summary + artifacts + (опционально) SARIF

Минимальная реализация (v1)

Action на composite/Node (проще)

скачивает ваш release бинарь evidra (или использует docker image)

запускает:

evidra ingest-findings --sarif ${{ inputs.sarif_path }} --session-id $GITHUB_RUN_ID

evidra scorecard --session-id $GITHUB_RUN_ID --format json

evidra validate --signatures

сохраняет evidence/ как artifact

пишет job summary (markdown) с risk level

Inputs для action.yml

session-id (default ${{ github.run_id }})

evidence-dir (default ./evidence)

sarif-path (optional)

fail-on-risk (optional: high|critical)

require-signatures (default true)

Outputs

risk_level

score

evidence_path

E2E тесты для Action (реально полезно)

Сделайте tests/e2e-action/:

запускаете workflow локально через act (опционально)

или просто e2e в CI:

uses: ./.github/actions/evidra

затем проверяете что артефакт загрузился, summary содержит строки

“3 задачи + Action” как единый roadmap (коротко)

v0.3.1

signing реально включён + validate --signatures

session_id проброшен в signal pipeline + scorecard per-session

ingest-findings CLI + entry type schema

e2e-1..3

v0.3.2

GitHub Action (composite) + пример workflow

e2e для action (минимум smoke)