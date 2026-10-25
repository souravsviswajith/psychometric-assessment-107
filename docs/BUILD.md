# Building and installing

## Requirements

| Tool | Version | Used for |
| --- | --- | --- |
| Go | 1.26+ (`go.mod` pins it; Go downloads the toolchain automatically) | runtime, tests, gomobile |
| Node.js | 20+ | building the web UI |
| JDK | 17 | Gradle / Android build |
| Android SDK | platform 36, build-tools 35+ | APK |
| Android NDK | r27 (any recent) | gomobile |

Set `ANDROID_HOME` (e.g. `~/Android/Sdk`). `ANDROID_NDK_HOME` is detected from
`$ANDROID_HOME/ndk/*` when not set.

## One command

```bash
scripts/build-apk.sh          # debug + release
scripts/build-apk.sh debug    # debug only
```

The script runs the Go tests, builds the web UI into `api/webdist`, binds the
Go runtime into `android/app/libs/career107.aar` with gomobile (arm64, armv7,
x86_64; Android 7.0+ / API 24), builds the APKs with Gradle, and copies them to
`dist/`.

## Step by step

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
go install golang.org/x/mobile/cmd/gobind@latest
gomobile init

(cd web && npm ci && npm run build)            # → api/webdist
gomobile bind -target=android/arm64,android/arm,android/amd64 -androidapi 24 \
  -javapkg com.career107 -o android/app/libs/career107.aar ./mobile
(cd android && ./gradlew assembleDebug)         # → android/app/build/outputs/apk/debug/
```

The UI must be built **before** `gomobile bind` (or `go build`), because the
Go binary embeds `api/webdist`.

## Installing on a phone

- Copy `dist/app-release.apk` (or `app-debug.apk`) to the phone and open it;
  allow "install unknown apps" for the file manager when Android asks.
- Or with USB debugging: `adb install -r dist/app-release.apk`.

## Signing for distribution

`app-release.apk` is currently signed with the Android **debug** key so it can
be installed for testing. Before publishing (Play Store or otherwise), create a
release keystore and replace `signingConfig signingConfigs.debug` in
`android/app/build.gradle` with a release signing config:

```bash
keytool -genkeypair -v -keystore career107-release.jks -alias career107 \
  -keyalg RSA -keysize 4096 -validity 10000
```

Keep the keystore and its passwords out of the repository.

## Running without Android

```bash
(cd web && npm ci && npm run build)
go run ./cmd/career107 -addr 127.0.0.1:8080 -data ./data
```

`-data` is where the session is saved; omit it to keep the session in memory.
For UI development, run the Go server and `npm run dev` in `web/` (Vite proxies
`/api` to port 8080).
