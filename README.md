# Annet Oil

Annet Oil - это Go-обертка для оркестрации нескольких annet контейнеров. Предоставляет CLI и REST API интерфейсы для управления annet операциями (gen, diff, patch, deploy) с автоматической маршрутизацией на основе hostname.

## Возможности

- 🐳 **Оркестрация Docker контейнеров** - управление несколькими annet контейнерами
- 🌐 **REST API** - HTTP API для интеграции с внешними системами
- 💻 **CLI интерфейс** - удобная командная строка с Cobra
- 🔀 **Автоматическая маршрутизация** - распределение команд по контейнерам на основе hostname
- 🔐 **SSH сервер** - удаленный доступ к командам
- ⚙️ **Гибкая конфигурация** - YAML конфигурация с поддержкой SSH ключей

## Архитектура

```
annet-oil (порт 22 SSH, 8080 API)
    ↓
JSON маршрутизация hostname → container
    ↓
┌─────────────────┬─────────────────┬─────────────────┐
│   annet-default │   annet-telnet  │   annet-orion   │
│   (по умолчанию)│   (telnet устр.)│   (orion устр.) │
└─────────────────┴─────────────────┴─────────────────┘
```

## Быстрый старт

### Установка

1. Клонируйте репозиторий:
```bash
git clone <repo-url>
cd annet-oil
```

2. Настройте окружение:
```bash
make setup
```

3. Соберите проект:
```bash
make build
```

### Docker

1. Запустите все сервисы:
```bash
make docker-run
```

2. Проверьте статус:
```bash
make docker-logs
```

## Использование

### CLI

```bash
# Генерация конфигураций
annet-oil gen -g router1.example.com
annet-oil gen -g device1,device2 --container annet-telnet

# Показать различия
annet-oil diff -G group1

# Применить изменения
annet-oil patch -g router1.example.com --dry-run
annet-oil deploy -g router1.example.com

# Управление контейнерами
annet-oil containers list
annet-oil routing show
annet-oil routing add device1.example.com annet-telnet

# Запуск серверов
annet-oil server start        # API + SSH
annet-oil server api          # только API
annet-oil server ssh          # только SSH
```

### REST API

```bash
# Генерация
curl -X GET "http://localhost:8080/api/v0/gen?filters=router1.example.com" \
  -H "Authorization: Bearer your-token"

# Развертывание с JSON
curl -X POST "http://localhost:8080/api/v0/deploy" \
  -H "Authorization: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "filters": ["router1.example.com"],
    "container": "annet-telnet",
    "dry_run": true
  }'

# Статус контейнеров
curl "http://localhost:8080/api/v0/containers" \
  -H "Authorization: Bearer your-token"

# Маршрутизация
curl "http://localhost:8080/api/v0/routing" \
  -H "Authorization: Bearer your-token"
```

### SSH

```bash
# Подключение по SSH
ssh -p 2222 admin@localhost

# Выполнение команд
ssh -p 2222 admin@localhost "annet-oil gen -g router1.example.com"
```

## Конфигурация

### configs/config.yaml

```yaml
annet_containers:
  - name: "annet"
    container_name: "annet-default"
    default: true
    description: "Default annet container"
  - name: "annet-telnet"
    container_name: "annet-telnet"
    description: "Telnet devices container"

ssh_keys:
  - name: "default"
    path: "/keys/id_rsa"
    user: "admin"

server:
  ssh:
    port: 22
    bind: "0.0.0.0"
  api:
    port: 8080
    bind: "0.0.0.0"
    auth_token: "your-secret-token"

storage:
  routing_file: "./storage/routing.json"

docker:
  # Для Docker Desktop: оставьте пустым (автоопределение)
  host: ""
  # Для Colima: unix:///Users/<user>/.colima/default/docker.sock
  # Для удаленного Docker: tcp://hostname:2376
  # api_version: "1.41"  # опционально
```

### storage/routing.json

```json
{
  "routes": {
    "router1.example.com": "annet",
    "old-router.example.com": "annet-telnet",
    "orion-device1.example.com": "annet-orion"
  }
}
```

## API Endpoints

| Endpoint | Методы | Описание |
|----------|--------|----------|
| `/api/v0/gen` | GET, POST | Генерация конфигураций |
| `/api/v0/diff` | GET, POST | Показать различия |
| `/api/v0/patch` | POST | Применить изменения |
| `/api/v0/deploy` | POST | Развернуть конфигурации |
| `/api/v0/containers` | GET | Статус контейнеров |
| `/api/v0/routing` | GET, POST, DELETE | Управление маршрутизацией |
| `/api/v0/health` | GET | Проверка здоровья |

## Makefile команды

```bash
make help           # Показать справку
make build          # Собрать проект
make run            # Запустить
make dev            # Режим разработки
make test           # Запустить тесты
make lint           # Проверить код
make docker-run     # Запустить в Docker
make clean          # Очистить артефакты
```

## Workflow

1. **Команда поступает** через CLI, API или SSH
2. **Парсинг параметров** - извлечение фильтров (-g, -G) и опций
3. **Маршрутизация** - определение целевого контейнера по hostname из routing.json
4. **Выполнение** - проксирование команды в соответствующий annet контейнер
5. **Возврат результата** - форматированный вывод пользователю

## Конфигурация Docker

### Docker Desktop
```yaml
docker:
  host: ""  # Автоопределение
```

### Colima
```yaml
docker:
  host: "unix:///Users/<username>/.colima/default/docker.sock"
  api_version: "1.41"
  tls_verify: false
```

### Удаленный Docker
```yaml
docker:
  host: "tcp://docker-host:2376"
  api_version: "1.41"
  tls_verify: true
  cert_path: "/path/to/certs"
```

### Быстрое переключение
```bash
# Для Colima
cp configs/config.colima.yaml configs/config.yaml

# Для Docker Desktop
cp configs/config.docker.yaml configs/config.yaml
```

## Переменные окружения

- `ANNET_OIL_CONFIG` - путь к конфигурационному файлу
- `DOCKER_HOST` - Docker daemon endpoint (переопределяет настройки конфига)
- `DOCKER_API_VERSION` - версия Docker API
- `DOCKER_CERT_PATH` - путь к TLS сертификатам
- `DOCKER_TLS_VERIFY` - включить TLS проверку

## Лицензия

MIT License