# CLAUDE.md — chaster-keyholder

Bot de IA en Go que actúa como "keyholder" dominante ("Papi") para la app de castidad Chaster. Gestiona sesiones de bloqueo via Telegram, asigna tareas diarias, ejecuta eventos de control, y mantiene una persona dominante/sexual en español.

---

## Stack

| Componente | Tecnología |
|-----------|-----------|
| Lenguaje | Go 1.23 |
| AI (texto) | Groq API — llama-3.3-70b-versatile |
| AI (visión) | Groq API — meta-llama/llama-4-scout-17b-16e-instruct |
| Bot interface | Telegram (go-telegram-bot-api/v5) |
| Scheduler | gocron/v2 |
| DB | SQLite (modernc.org/sqlite, WAL mode) |
| Imágenes | Cloudinary |
| Chaster API | Cliente custom (2 endpoints: Public + Extensions) |
| Web | net/http — dashboard con Basic Auth |

---

## Estructura de paquetes

```
main.go              — entrypoint, wiring, landing page en PORT (default 8080)
ai/keyholder.go      — todos los prompts y llamadas Groq (~1,868 líneas)
telegram/bot.go      — lógica del bot, AppState en memoria, handlers (~4,147 líneas)
scheduler/jobs.go    — cron jobs (~217 líneas)
models/types.go      — ChasterLock, Task, Toy, ActiveEvent, AppState, IntensityLevel
storage/db.go        — SQLite: schema, migraciones, CRUD (~1,387 líneas)
storage/cloudinary.go — helpers de upload de imágenes
chaster/client.go    — cliente REST de Chaster (~837 líneas)
web/server.go        — dashboard HTTP con template rendering
```

---

## Concurrencia y mutexes

**Tres mutexes con responsabilidades distintas — NO mezclar:**

- `handlerMu` — serializa TODAS las mutaciones de `b.state`. Tanto el scheduler como el loop de Telegram deben envolverse en este mutex. El scheduler siempre usa `bot.WithLock(func() {...})`.
- `stateMu` — protege escrituras atómicas a state.json y el caché de `daysLocked` (separado para no bloquear rate limit de chat).
- `chatMu` — rate-limit del chat libre (1 mensaje / 3 segundos).

**Regla crítica**: nunca acceder a `b.state` fuera de `handlerMu` desde goroutines del scheduler.

---

## Persistencia de estado (dual-layer)

**Primario — state.json:**
- Escritura atómica: escribe a `state.json.tmp` → renombra (operación atómica en Linux)
- Contiene: CurrentTask, flags de ritual/plug/outfit, ActiveEvent, counters, TasksStreak, WeeklyDebt

**Backup — tabla `session_state` (SQLite):**
- Una sola fila `id='current'`
- Upsert en cada saveState()
- Fuente de verdad para counters si state.json está corrupto

**Recuperación al startup (loadState):**
1. Lee state.json → si OK, usa esos datos
2. Si falta o inválido → carga desde DB
3. Si state.json existe pero counters son 0 y DB disponible → restaura counters de DB
4. **Toys SIEMPRE cargados desde DB** (no desde state.json)

**No persistido (se reconstruye al startup):**
- `pendingActions` — reconstruido desde flags del estado
- `pendingToyHint` — transitorio

---

## Cola de pendingActions

FIFO con mapa de prioridades. Las acciones persistentes se insertan por prioridad:
- `ritual_photo` = 0 (máxima prioridad)
- `plug_photo` = 2
- `outfit_photo` = 4

Las acciones efímeras (`new_toy`, `selecting_cage`) se insertan al frente siempre. Sin duplicados. Se desencola al completar con `removePendingAction()`.

**Toda foto recibida se clasifica por `currentPendingAction()`** — nunca por heurística del contenido.

---

## Intensidad y obediencia

**IntensityLevel** (escala tareas/penalizaciones):
- Light: < 4 días bloqueada
- Moderate: 4–8 días
- Intense: 8–15 días
- Maximum: 15+ días

**ObediencePoints** (TasksStreak — nombre confuso, es acumulador de por vida):
- +1 tarea completada (+2 si 8+ días bloqueada)
- +3 bonus cada 7 días consecutivos
- +1 por cada 2 confirmaciones de plug (via PlugBonusAccum)
- -3 por tarea fallada
- -1 por edge no confirmado en 2 horas
- **Nunca resetea a 0**

**ObedienceTitle** (GetObedienceLevelFromPoints): 0/4/9/15/21+ puntos → 5 niveles

**ConsecutiveDays** (para el bonus de 7 días):
- Incrementa con cada tarea completada
- Se resetea a 0 si pasan 2+ días sin completar tarea (CheckObedienceDecay, 11pm diario)

---

## Manejo de errores

**No reintentable**: HTTP 4xx (excepto 429) → falla inmediata con `nonRetryableError`

**Reintentable**: errores de red + HTTP 429/5xx → backoff exponencial 1s/2s, máximo 3 intentos

**Fallbacks en cascada**:
- RemoveTime: intenta Extensions API → si falla, public API con valor negativo
- Carga de estado: state.json → DB → estado vacío
- Parseo de Markdown: si Telegram rechaza → reenviar como texto plano

**Fallos silenciosos** (loguean pero no bloquean): Cloudinary upload/delete, AI non-critical

---

## Scheduler (todos en COT = America/Bogota, UTC-5)

| Hora | Job |
|------|-----|
| 8:00 | SendMorningStatus |
| 8:30 | StartMorningRitual |
| 8:45 | SendPlugAssignment |
| 9:00 | HandleTask |
| 10:00 | SendDailyOutfit + SendConditioningMessage |
| 11:00 | CheckRitualExpiry |
| 12:00, 16:00, 20:00 | SendRandomMessage |
| 14:00 | SendConditioningMessage |
| 18:00 | HandleRuleta |
| 22:00 | SendNightStatus |
| 23:00 | CheckObedienceDecay |
| Cada 1 min | CheckLockFinished |
| Cada 5 min | CheckActiveEventExpiry, CheckCheckinExpiry, CheckPlugReminder |
| Cada 15 min | CheckChasterTaskVote |
| Cada 30 min (8-22h) | HandleRandomEvent |
| Cada 45min-3h (async) | TriggerCheckin (loop separado) |
| Domingos 21:00 | HandleWeeklyJudgment |

---

## Comandos activos en el switch

`/status`, `/task`, `/explain`, `/fail`, `/roulette`, `/chatask`, `/newlock [duration]`,
`/contract`, `/toys`, `/wardrobe`, `/lockstats`, `/history`, `/permissions`, `/came [method]`,
`/stats`, `/mood`, `/removetime`, `/order [level]`, `/help`

**Comandos desactivados (código existe, NO en el switch):**
- `/dbwipe` → `HandleDBWipe()`
- `/testevent` → `HandleRandomEventTest()`

---

## Tablas SQLite relevantes

`toys`, `locks`, `tasks`, `chaster_tasks`, `clothing`, `outfit_log`, `events`,
`negotiations`, `permission_log`, `orgasm_log`, `session_state`, `contracts`,
`contract_rules`, `checkins`, `chat_history`, `violations_log`, `schema_version`

**Migraciones**: versión 1 (schema completo) + versión 2 (renombrados + columnas). Agregar nuevas migraciones siempre al final.

---

## AI — Patrones de prompts (ai/keyholder.go)

**Dos system prompts base:**
- `baseSystemLocked` — Papi dominante/sexual cuando está bloqueada
- `baseSystemFree` — Papi impaciente/avergonzante cuando está libre

**Siempre inyectar `buildContext()`**: días bloqueada, intensidad, estado de juguetes.

**Extracción de JSON**: usar `extractJSON()` regex — los modelos a veces envuelven JSON en texto. Siempre tener defaults seguros (mínimo 12h para duración, "denied" para orgasmos).

**Verificación de fotos DESACTIVADA**: `VerifyTaskPhoto` existe pero no se llama. Las fotos se auto-aceptan y suben a Cloudinary. No reactivar sin revisar el flujo completo — había alta tasa de falsos negativos.

**Vision**: codificación base64 dataURL. Criterio generoso ("aprobar cuando haya duda").

---

## Chaster API (chaster/client.go)

**Dual-API:**
- **Public API** — token de usuario, operaciones principales
- **Extensions API** — dev token + slug, operaciones avanzadas (RemoveTime preferido aquí)

**GetSessionByLockID()**: busca en la lista de ExtensionSession por lockId (revisa tanto `LockID` como `Lock.ID`). Retorna `sessionId`, no `_id`.

**Parseo de fechas**: intenta 3 formatos ISO 8601 (con/sin milisegundos).

**ErrLockNotFound**: 404 = lock ya archivado/desbloqueado — manejar gracefully.

---

## Quirks importantes

**RitualStep**: máquina de estados (0=no iniciado, 1=esperando foto, 2=esperando mensaje). A las 11am: si != 0, penalizar 1 hora y limpiar.

**Edge timeout**: 2 horas para confirmar con `/came`. Si no → -1 ObediencePoint + deuda semanal + posible pillory automático. Alerta a los 30 min.

**ActiveEvent auto-reversión**: Freeze/HideTime/Pillory tienen `ExpiresAt`. Cada 5 min `CheckActiveEventExpiry` los elimina al expirar. No hay llamada API explícita de "unfreeze" — el tiempo de la extensión en Chaster expira solo.

**PlugBonusAccum**: contador 0-1. Cada 2 confirmaciones de plug exitosas → +1 ObediencePoint, reset a 0.

**WeeklyJudgment (domingos 21:00)**: suma WeeklyDebt (infracciones de la semana) → consecuencias variables → resetea WeeklyDebt y LastJudgmentDate.

**daysLocked caché**: se cachea 5 minutos para evitar llamadas repetidas al scheduler (corre cada 30s).

**Makefile mismatch**: `make run` apunta a `./cmd/bot` pero `main.go` está en la raíz. Usar `go run .` en desarrollo.

---

## Variables de entorno

**Obligatorias:**
```
CHASTER_TOKEN, TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID, GROQ_API_KEY,
CLOUDINARY_CLOUD_NAME, CLOUDINARY_API_KEY, CLOUDINARY_API_SECRET
```

**Opcionales:**
```
CHASTER_EXTENSION_TOKEN, CHASTER_EXTENSION_SLUG  — para freeze/pillory/hidetime
DB_PATH                                            — default: keyholder.db
PORT                                               — default: 8080
TELEGRAM_BOT_USERNAME
```

---

## Reglas para modificar el código

1. **Nuevos comandos**: agregar al switch en `telegram/bot.go` Y al teclado/help si son públicos.
2. **Nuevas tablas**: agregar como migración nueva en `storage/db.go` (versionada, al final).
3. **Nuevo estado persistente**: agregar al struct `AppState` en `models/types.go`, al JSON marshal, Y al upsert de `session_state` si es un counter crítico.
4. **Jobs del scheduler**: siempre envolver mutaciones de estado en `bot.WithLock()`.
5. **Fechas**: almacenar en UTC; convertir a COT solo para display y comparaciones de día.
6. **No romper la cola pendingActions**: respetar el mapa de prioridades al agregar nuevos tipos de acción persistente.
