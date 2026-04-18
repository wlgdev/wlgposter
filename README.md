<!--suppress HtmlUnknownAnchorTarget, HtmlDeprecatedAttribute -->
<div id="top"></div>

<div align="center">
  <h1 align="center">wlgposter</h1>

  <p align="center">
    A Telegram repost bot that mirrors selected channel posts to VK and MAX.
    It keeps target posts in sync for new messages, edits, and deletions.
  </p>
</div>

<div align="center">
  📦 Telegram -> VK / MAX
</div>
<div align="center">
  <img src="./docs/description.webp" alt="wlgposter description"/>
</div>

<!-- TABLE OF CONTENT -->
<details>
  <summary>Table of Contents</summary>
  <ol>
    <li>
      <a href="#-description">📃 Description</a>
      <ul>
        <li><a href="#built-with">Built With</a></li>
      </ul>
    </li>
    <li>
      <a href="#-getting-started">🪧 Getting Started</a>
      <ul>
        <li><a href="#prerequisites">Prerequisites</a></li>
        <li><a href="#installation">Installation</a></li>
        <li><a href="#environment-variables">Environment Variables</a></li>
      </ul>
    </li>
    <li>
      <a href="#%EF%B8%8F-how-to-use">⚠️ How to use</a>
      <ul>
        <li><a href="#possible-exceptions">Possible Exceptions</a></li>
      </ul>
    </li>
    <li><a href="#%EF%B8%8F-deployment">⬆️ Deployment</a></li>
    <li><a href="#-reference">🔗 Reference</a></li>
  </ol>
</details>

<br>

## 📃 Description
`wlgposter` listens to one or more Telegram channels and republishes posts to VK and MAX.

The service is designed to keep destination platforms synchronized with the source channel:

- publishes new Telegram posts
- updates target posts when the original Telegram message is edited
- removes target posts when the original Telegram post is deleted
- supports media attachments, albums, replies, and inline keyboards
- stores post/media cache in SQLite to avoid duplicate publishing
- supports Telegram admin commands for health checks and banned words management

Posts are processed only from channels listed in `TELEGRAM_TARGET_CHANNEL_ID`. The bot also supports per-platform banned words, so a post can be skipped for VK, MAX, or both.

<p align="right">(<a href="#top">back to top</a>)</p>

### Built With
- [Go 1.26](https://go.dev/)
- [gogram](https://github.com/amarnathcjd/gogram)
- [VK SDK for Go](https://github.com/SevereCloud/vksdk)
- [MAX Bot API Client for Go](https://github.com/max-messenger/max-bot-api-client-go)
- [SQLite](https://www.sqlite.org/index.html)
- [OpenTelemetry](https://opentelemetry.io/)
- [Docker Compose](https://docs.docker.com/compose/)

## 🪧 Getting Started
You can run the service locally with Go or inside Docker. In both cases the bot needs valid Telegram, VK, and MAX credentials.

### Prerequisites
- [Go 1.26+](https://go.dev/dl/)
- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/) for containerized runs
- Telegram API credentials (`API_ID`, `API_HASH`)
- A Telegram bot token with access to the source channel(s)
- VK and MAX bot credentials for the target destinations
- An OTLP-compatible endpoint for logs and metrics

### Installation
1. Clone the repository.
2. Create either `.env.dev` for local development or `.env.prod` for production-like runs.
3. Fill in all required environment variables listed below.
4. Start the service:

```bash
go run ./cmd/main.go
```

For Docker Compose runs, set `SERVICE_IMAGE_NAME` and `SERVICE_IMAGE_TAG` first. In deployment they are injected by GitHub Actions; for manual runs you can point them to a published GHCR image. Also make sure the external network `monitor_default` already exists:

```bash
docker network create monitor_default
```

Then start the stack:

```bash
docker compose up -d
```

### Environment Variables
The required runtime variables are defined in [`docker-compose.yml`](./docker-compose.yml).

| Variable | Required | Description |
| --- | --- | --- |
| `SERVICE_IMAGE_NAME` | for Docker deploy | Docker image name used by Compose |
| `SERVICE_IMAGE_TAG` | for Docker deploy | Docker image tag used by Compose |
| `ENV` | yes | Application environment, for example `development` or `production` |
| `API_ID` | yes | Telegram application API ID |
| `API_HASH` | yes | Telegram application API hash |
| `TELEGRAM_BOT_TOKEN` | yes | Bot token used to read Telegram updates |
| `TELEGRAM_TARGET_CHANNEL_ID` | yes | Comma-separated list of Telegram channel IDs to monitor |
| `TELEGRAM_ADMINS` | yes | Comma-separated list of Telegram user IDs allowed to use admin commands |
| `MAX_BOT_TOKEN` | yes | MAX bot token used for publishing |
| `MAX_TARGET_CHAT_ID` | yes | Target MAX chat ID |
| `VK_TOKEN` | yes | VK token used for publishing |
| `VK_GROUP_ID` | yes | VK group ID where posts are published |
| `FILE_SIZE_LIMIT` | yes | Max allowed media file size |
| `LOG_LEVEL` | yes | Log level, for example `debug` or `info` |
| `OTLP_ENDPOINT` | yes | OTLP HTTP endpoint, for example `localhost:4318` |

The following paths are also used by the application, but in Docker they are already set by Compose:

| Variable | Description |
| --- | --- |
| `TMP_DIR` | Temporary directory for downloaded media files |
| `DB` | Directory where SQLite database and Telegram session/cache files are stored |

<p align="right">(<a href="#top">back to top</a>)</p>

## ⚠️ How to use
After startup, the bot subscribes to Telegram updates and mirrors posts from the configured channels.

Basic workflow:

1. Add the bot to the source Telegram channel and ensure it can read posts.
2. Set `TELEGRAM_TARGET_CHANNEL_ID` to the allowed source channel IDs.
3. Configure VK and MAX destination credentials.
4. Start the service and publish posts in Telegram.
5. The bot will create, update, or delete mirrored posts on target platforms automatically.

Available Telegram admin commands:

- `/ping` - health check command
- `/banword list` - show banned words
- `/banword add <word>` - block a word on both VK and MAX
- `/banword add_vk <word>` - block a word only for VK
- `/banword add_max <word>` - block a word only for MAX
- `/banword delete <word>` - remove a banned word

### Possible Exceptions
- `monitor_default` is declared as an external Docker network. Create it before running Compose if it does not exist.
- If `OTLP_ENDPOINT` is unreachable, observability starts in degraded mode.
- Telegram channels not listed in `TELEGRAM_TARGET_CHANNEL_ID` are ignored.
- Single audio-only or voice-only posts are skipped for VK because that target does not support them in the current implementation.
- Missing or invalid platform credentials will prevent publishing to the corresponding destination.

<p align="right">(<a href="#top">back to top</a>)</p>

## ⬆️ Deployment
Deployment is performed by publishing a GitHub Release in this repository.

Release flow:

1. Create and publish a new GitHub Release.
2. The workflow [`.github/workflows/publish-release.yml`](./.github/workflows/publish-release.yml) builds the Linux binary.
3. The same workflow builds and pushes the Docker image to GHCR.
4. After that, the workflow deploys the updated image to the target host with `docker compose`.

The deployment workflow injects runtime values from GitHub repository secrets. At minimum, configure:

- SSH access secrets: `WLGDEV_SSH_PRIVATE_KEY`, `WLGDEV_SSH_PRIVATE_HOST`, `WLGDEV_SSH_PRIVATE_PORT`, `WLGDEV_SSH_PRIVATE_USER`
- Application secrets: `API_ID`, `API_HASH`, `TELEGRAM_BOT_TOKEN`, `TELEGRAM_TARGET_CHANNEL_ID`, `TELEGRAM_ADMINS`, `MAX_BOT_TOKEN`, `MAX_TARGET_CHAT_ID`, `VK_TOKEN`, `VK_GROUP_ID`, `FILE_SIZE_LIMIT`, `LOG_LEVEL`, `OTLP_ENDPOINT`

During deployment, the workflow also sets:

- `SERVICE_IMAGE_NAME=ghcr.io/<owner>/<image>`
- `SERVICE_IMAGE_TAG=<release-tag>`
- `ENV=production`

The remote host must have Docker, Docker Compose, and the external network `monitor_default` available.

<p align="right">(<a href="#top">back to top</a>)</p>

## 🔗 Reference
- [Go Documentation](https://go.dev/doc/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Telegram Bot API](https://core.telegram.org/bots/api)
- [gogram](https://github.com/amarnathcjd/gogram)
- [VK API](https://dev.vk.com/)
- [MAX Bot API Client for Go](https://github.com/max-messenger/max-bot-api-client-go)
- [OpenTelemetry](https://opentelemetry.io/docs/)

<p align="right">(<a href="#top">back to top</a>)</p>
