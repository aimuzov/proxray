<p align="center">
  <img src="docs/demo.gif" alt="proxray в работе" width="900" />
</p>

# proxray

[English](README.md) | **Русский**

Терминальный VPN-клиент, совместимый с профилями подписок [HAPP](https://happ.su).
Забирает подписку, парсит share-ссылки (VLESS / VMess / Trojan / Shadowsocks) и
поднимает соединение через встроенный
[xray-core](https://github.com/XTLS/Xray-core) — как локальный прокси, системный
прокси или полноценный системный VPN (TUN).

Единый самодостаточный бинарник: xray-core и tun2socks встроены, внешних бинарей
не требуется.

## Возможности

- **Совместимость с HAPP** в обоих форматах, которые отдают панели:
  base64-список share-ссылок и JSON-подписка (готовые конфиги xray). Читаются
  метаданные из заголовков, которые понимает HAPP:
  `subscription-userinfo` (трафик/срок), `profile-title`,
  `profile-update-interval`, `support-url`. Запросы уходят с
  `User-Agent: Happ/1.0`, чтобы панель вернула формат, который ждёт HAPP
  (переопределяется флагом `--ua`).
- **JSON-подписка исполняется так, как её написала панель**: правила
  маршрутизации, балансировщики и автопереключение по observatory сохраняются
  без изменений, своими остаются только порты прослушивания и уровень логов.
  Маршрутизацию задаёт панель, поэтому `--bypass` к таким записям не
  применяется.
- **Протоколы**: VLESS (включая Reality / XTLS Vision), VMess, Trojan,
  Shadowsocks, Hysteria2. **Транспорты**: TCP, WebSocket, gRPC, HTTP/2.
  **Безопасность**: TLS, Reality.
- **Три способа завернуть трафик**:
  - `connect` — локальный SOCKS5 + HTTP прокси на `127.0.0.1` (без root);
  - `connect --system-proxy` — выставляет системный SOCKS + HTTP/HTTPS прокси
    macOS (нужен `sudo`), чтобы браузеры и большинство приложений шли через него
    без правки таблицы маршрутов — уживается с другим активным VPN;
  - `connect --mode tun` — полноценный системный VPN через utun (нужен `sudo`).

> **Примечание**: xray-core не умеет TUIC, а также Hysteria2 с обфускацией.
> Такие серверы всё равно парсятся и показываются (с пометкой `unsupported`), но
> подключиться к ним через xray нельзя (нужен движок на базе sing-box).

## Как это устроено

```
subscription URL
      │  profile.Fetch (User-Agent: Happ/1.0)
      ▼
base64-список ссылок ──► link.Parse ──► []link.Server
                                            │ xray.BuildConfig
   JSON-конфиги ────────► rawconf.Parse ────┤ rawconf.Render
                                            ▼
                                    конфиг xray-core (JSON)
                                            │ xray.Start (встроенное ядро)
              ┌─────────────────────────────┼─────────────────────────────┐
              ▼                             ▼                             ▼
      proxy: SOCKS5/HTTP          --system-proxy: networksetup     tun: tun2socks
      на 127.0.0.1                ставит системный SOCKS/HTTP       + таблица маршрутов
      (без root)                  (sudo)                           (sudo, utun)
```

## Установка

### mise (рекомендуется)

Готовые бинарники публикуются в GitHub Releases. Ставятся через
[mise](https://mise.jdx.dev) — без установки Go:

```sh
mise use -g "github:aimuzov/proxray@latest"
```

или зафиксировать в `mise.toml`:

```toml
[tools]
"github:aimuzov/proxray" = "latest"
```

Бэкенд `ubi` работает так же и с теми же релизами, если он тебе привычнее:
`ubi:aimuzov/proxray`.

> При частых установках задай `MISE_GITHUB_TOKEN` (или `GITHUB_TOKEN`), чтобы не
> упереться в лимиты GitHub API.

### Ручная загрузка

Скачай архив под свою ОС/архитектуру со страницы
[Releases](https://github.com/aimuzov/proxray/releases), распакуй и положи
бинарник `proxray` в `PATH`.

### Из исходников

```sh
git clone https://github.com/aimuzov/proxray
cd proxray
go build -o proxray .   # нужен Go 1.26+
```

Полученный бинарник `proxray` самодостаточен.

> **`go install github.com/aimuzov/proxray@latest` не работает.** Сборке нужна
> директива `replace` в `go.mod` (примиряет xray-core и tun2socks по gvisor), а
> `go install pkg@version` игнорирует `replace`. Используй готовый бинарник либо
> клонируй и собирай.

## Использование

### Подписки

```sh
proxray sub add https://panel.example/sub/TOKEN --name myvpn   # добавить (станет активной)
proxray sub list                                               # список подписок
proxray sub update [name]                                      # обновить (по умолчанию активную)
proxray sub use <name>                                         # сделать подписку активной
proxray sub rm <name>                                          # удалить
```

`sub list` показывает трафик и срок из заголовков подписки:

```
ACTIVE  NAME    TITLE       SERVERS  TRAFFIC          EXPIRES
*       myvpn   My VPN      12       12.4 GB / 200 GB  2026-09-01
```

### Серверы

```sh
proxray list           # серверы активной подписки
proxray list --sub x   # серверы конкретной подписки
```

```
#  PROTOCOL   ADDRESS         TAG               NOTE
1  vless      4 servers       🇪🇺 Авто           Выбирает быстрый сервер
2  vless      de.example:443  🇩🇪 Германия
3  hysteria   hy.example:443  Fast HY2
```

Записи JSON-подписки показывают описание от панели, а запись с пулом серверов за
балансировщиком — их количество вместо одного адреса. Колонка `NOTE` не
выводится, если описаний в подписке нет.

### Подключение

`connect` работает в foreground до прерывания `Ctrl+C`. Аргумент `selector`
выбирает сервер: пусто = первый, число = индекс (1-based) из `proxray list`, либо
подстрока тега без учёта регистра.

```sh
proxray connect                 # первый сервер, режим proxy
proxray connect 2               # сервер №2
proxray connect germany         # первый сервер с тегом, содержащим "germany"

sudo proxray connect 1 --system-proxy   # браузеры/приложения через системный прокси (без правки маршрутов)
sudo proxray connect 1 --mode tun       # полноценный системный VPN
```

В обычном proxy-режиме настрой приложения на `socks5://127.0.0.1:10808`
(в Firefox включи «Proxy DNS when using SOCKS v5»).

### Флаги `connect`

| Флаг             | По умолчанию | Назначение                                                 |
| ---------------- | ------------ | ---------------------------------------------------------- |
| `-m, --mode`     | `proxy`      | `proxy` или `tun`                                          |
| `--socks`        | `10808`      | порт локального SOCKS5                                     |
| `--http`         | `10809`      | порт локального HTTP (режим proxy)                         |
| `--system-proxy` | `false`      | выставить системный прокси macOS (режим proxy, нужен sudo) |
| `--sub`          | активная     | имя подписки                                               |
| `--bypass`       | `ru`         | гнать трафик региона напрямую: `ru` или `off`              |

### Три способа завернуть трафик — сравнение

- **`connect` (proxy)** — только приложения, явно настроенные на
  `socks5://127.0.0.1:10808` (например, Firefox с remote DNS). Без root.
- **`connect --system-proxy`** — выставляет на всех включённых сетевых сервисах
  системный SOCKS (порт `--socks`) и HTTP/HTTPS (порт `--http`), поэтому
  Safari/Chrome и приложения, игнорирующие SOCKS, идут через прокси. **Не трогает**
  таблицу маршрутов, поэтому **уживается с другим активным VPN**. Нужен `sudo`;
  прежние настройки прокси восстанавливаются при выходе. Если сессию убили
  (`kill -9`) и прокси завис — сбросить командой `sudo proxray system-proxy off`.
- **`connect --mode tun`** — полноценный системный VPN через utun, перехватывает
  весь трафик. Нужен `sudo`. Если параллельно активен другой VPN — сначала
  отключи его, чтобы туннели не дрались за маршруты/DNS.

### Обход российского трафика

По умолчанию proxray направляет российские домены и диапазоны IP
(`geosite:category-ru`, `geoip:ru`) напрямую, мимо туннеля, чтобы сайты,
блокирующие иностранные VPN (например `ozon.ru`), продолжали работать. При первом
подключении базы `geoip.dat` и `geosite.dat` скачиваются из
[Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat)
в `<config>/geo/` и обновляются раз в сутки. Обход действует в режимах proxy и
`--system-proxy`. В режиме tun он **пока не поддерживается** (сокеты
direct-outbound всё равно зацикливаются обратно в utun), поэтому
`connect --mode tun` принудительно отключает обход с предупреждением и гонит весь
трафик через туннель.

```sh
proxray connect --bypass off    # весь трафик через туннель (на один запуск)
proxray connect --bypass ru     # принудительный обход РФ (на один запуск)

proxray route                   # показать настройку обхода активной подписки
proxray route set off           # сохранить: весь трафик через туннель
proxray route set ru            # сохранить: обходить российский трафик (по умолчанию)
proxray route update            # принудительно обновить geo-базы
```

Флаг `--bypass` переопределяет сохранённую настройку на один запуск; `route set`
меняет сохранённое значение по умолчанию для подписки. Если при включённом обходе
базы скачать не удалось, `connect` завершается с понятной ошибкой, а не отправляет
РФ-трафик в туннель молча — повтори при наличии сети или используй `--bypass off`.

### Прочие команды

```sh
proxray config [selector]       # вывести сгенерированный конфиг xray-core (отладка)
proxray system-proxy off        # аварийный сброс системного прокси (sudo)
```

## Конфигурация и хранение

Состояние (подписки и кэш ссылок) хранится в `state.json` в конфиг-каталоге
пользователя (`~/Library/Application Support/proxray` на macOS),
переопределяется глобальным флагом `--home`.

## Детали режима TUN (macOS)

1. адрес сервера резолвится в IP, и на каждый добавляется host-маршрут к текущему
   next-hop (физический шлюз либо интерфейс уже активного VPN) — чтобы соединение
   прокси с сервером не зациклилось обратно в туннель;
2. создаётся устройство `utun`, и tun2socks форвардит его трафик на локальный
   SOCKS, который отдаёт xray;
3. дефолтный маршрут перекрывается двумя `/1`-маршрутами на utun (реальный default
   остаётся нетронутым для корректного отката);
4. глобальный IPv6 заворачивается в `lo0` (блокируется), чтобы IPv6-сайты
   (Google, YouTube) не утекали мимо туннеля — приложения откатываются на IPv4
   через туннель; link-local IPv6 продолжает работать по более специфичному
   маршруту;
5. при `Ctrl+C` все маршруты снимаются в обратном порядке.

## Ограничения

- **TUIC**, а также **Hysteria2 с обфускацией**, xray-core не умеет (парсятся и
  показываются как `unsupported`).
- **`--bypass` не применяется к записям JSON-подписки**: маршрутизацию задают
  правила самой панели.
- **TUN и `--system-proxy` пока только для macOS**.
- **В режиме TUN IPv6 заблокирован** (путь прокси — IPv4); IPv6-only ресурсы во
  время подключения недоступны.
- `connect` работает в **foreground**; фонового демона пока нет.
- `kill -9` пропускает очистку: системный прокси останется включённым
  (`sudo proxray system-proxy off`), а IPv6-блок-маршруты TUN — в таблице
  (`sudo route -n delete -inet6 -net ::/1; sudo route -n delete -inet6 -net
8000::/1`). Обычный `Ctrl+C` всё убирает сам.

## Структура проекта

| Пакет               | Назначение                                                   |
| ------------------- | ------------------------------------------------------------ |
| `internal/link`     | парсинг share-ссылок (vless/vmess/trojan/ss/hysteria2)       |
| `internal/profile`  | загрузка подписки, декод тела + заголовков                    |
| `internal/rawconf`  | JSON-подписки: разбор и правка готовых конфигов               |
| `internal/xray`     | сборка конфига xray-core из сервера, запуск встроенного ядра |
| `internal/tunnel`   | режим TUN: tun2socks + управление маршрутами macOS           |
| `internal/sysproxy` | системный прокси macOS через networksetup                    |
| `internal/store`    | хранение подписок и кэша ссылок                              |
| `internal/cli`      | команды cobra                                                |

## Разработка

```sh
go test ./...        # юнит-тесты + реальный end-to-end тест прокси
go vet ./...
```

Интеграционный тест xray поднимает реальный Shadowsocks-сервер и клиента,
собранного из `link.Server`, и проверяет, что HTTP-запрос через SOCKS-inbound
клиента доходит до цели сквозь прокси.

> xray-core и tun2socks требуют разные версии `gvisor.dev/gvisor`; директива
> `replace` в `go.mod` фиксирует gvisor на версии, с которой собираются оба. Не
> удаляй её — см. комментарий рядом.

### Запись демо

Гифка в шапке записана скриптом, так что её можно переснять на любой машине с
[vhs](https://github.com/charmbracelet/vhs) и `ttyd`:

```sh
go build -o proxray .
cd docs && vhs demo.tape
```

Запись идёт против локальной фейковой панели (`docs/demo/panel.go` отдаёт
вымышленную подписку на `127.0.0.1:8099`), в одноразовом fish-профиле
`docs/demo-profile` и с `HOME`, указывающим на `docs/demo-home`. Поэтому команды
в кадре настоящие, а настоящая подписка — ничья: ни URL, ни серверы рекордера в
гифку не попадают. Первая запись скачивает geo-базы для RU-байпаса; дальше они
берутся из кэша.

### Релизы

Релизы собирает [GoReleaser](https://goreleaser.com) в CI по пушу тега:

```sh
git tag v0.1.0
git push origin v0.1.0
```

Workflow `release` (`.github/workflows/release.yml`) собирает бинарники
darwin/linux под amd64/arm64 и публикует их в GitHub Releases. Там сборка
учитывает `replace` из `go.mod` (proxray — главный модуль). Локальный прогон:
`goreleaser release --clean --snapshot`.
