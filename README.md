# 🔒 Chaster Keyholder Bot

Bot de Telegram con IA que actúa como keyholder virtual para sesiones de castidad en Chaster.app.

## Funciones

- 🌅 **Status matutino automático** — 8:00 AM con mensaje del keyholder IA
- 🌙 **Status nocturno automático** — 10:00 PM con resumen del día
- 📋 **Tareas diarias** — generadas por IA, con recompensas y penalizaciones reales en Chaster
- 🎲 **Minijuegos obligatorios** — dados cada 6-18 horas (random), con 30 min para jugar o penalización automática
- 🤖 **Keyholder virtual** — IA que manipula tu tiempo de castidad según tu comportamiento

## Tabla de dados

| Total | Resultado |
|-------|-----------|
| 2-3   | +2 horas  |
| 4-5   | +1 hora   |
| 6-7   | Sin cambio|
| 8-9   | -30 min   |
| 10-11 | -1 hora   |
| 12    | -2 horas  |

## Comandos del bot

| Comando | Descripción |
|---------|-------------|
| `/status` | Ver estado actual |
| `/tarea` | Ver o solicitar tarea del día |
| `/completar` | Marcar tarea completada |
| `/fallar` | Confesar que fallaste |
| `/jugar` | Tirar dados (cuando sea obligatorio) |
| `/help` | Ayuda |

## Setup local

### 1. Clonar y configurar

```bash
git clone <tu-repo>
cd chaster-keyholder
cp .env.example .env
# Editar .env con tus tokens
```

### 2. Obtener credenciales

**Chaster Token:**
1. Ve a https://chaster.app/settings/api
2. Crea una nueva aplicación
3. Copia el token

**Telegram Bot Token:**
1. Escríbele a @BotFather en Telegram
2. Usa `/newbot` y sigue las instrucciones
3. Copia el token

**Tu Chat ID:**
1. Escríbele a @userinfobot en Telegram
2. Te responde con tu ID

**Groq API Key:**
1. Ve a https://console.groq.com
2. Crea una cuenta
3. Ve a API Keys → Create API Key

### 3. Ejecutar localmente

```bash
make tidy     # instalar dependencias
make run      # correr en desarrollo
make build    # compilar binario en bin/
```

## Deploy en Railway

### 1. Subir a GitHub

```bash
git init
git add .
git commit -m "initial commit"
git remote add origin <tu-repo>
git push -u origin main
```

### 2. Crear proyecto en Railway

1. Ve a https://railway.app
2. **New Project** → **Deploy from GitHub repo**
3. Selecciona tu repositorio
4. Railway detecta el Dockerfile automáticamente

### 3. Configurar variables de entorno en Railway

En el panel de Railway → Variables:

```
CHASTER_TOKEN=tu_token
TELEGRAM_BOT_TOKEN=tu_bot_token
TELEGRAM_CHAT_ID=tu_chat_id
GROQ_API_KEY=gsk_tu_key
```

### 4. Volumen persistente (importante)

Para que el `state.json` persista entre reinicios:

1. En Railway → tu servicio → **Volumes**
2. Añadir volumen: mount path `/app`
3. Redeploy

¡Listo! El bot corre 24/7.

## Estructura del proyecto

```
chaster-keyholder/
├── cmd/
│   └── bot/
│       └── main.go              # Entrada principal
├── internal/
│   ├── ai/
│   │   └── keyholder.go         # Keyholder virtual (Groq)
│   ├── chaster/
│   │   └── client.go            # Cliente API de Chaster
│   ├── models/
│   │   └── types.go             # Tipos compartidos
│   ├── scheduler/
│   │   └── jobs.go              # Tareas automáticas
│   └── telegram/
│       └── bot.go               # Bot y comandos
├── deployments/
│   └── Dockerfile               # Para Railway
├── .env.example
├── Makefile
└── README.md
```
