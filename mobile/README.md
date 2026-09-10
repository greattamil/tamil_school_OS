# Tamil School OS — Mobile

Flutter app for teachers and parents (PRD section 8.1: Android first). See
[docs/PROGRESS.md](../docs/PROGRESS.md) for what's implemented and verified so far.

## Running locally

The backend must already be up (`docker compose up -d` from the repo root).

```
flutter pub get
flutter run -d <device>
```

The default API base URL (`http://10.0.2.2:8080`) is the standard Android-emulator
alias for the host machine's `localhost`, so it reaches the Dockerized backend with
no extra configuration when running on an emulator. Override it for a real device
or a non-default backend:

```
flutter run --dart-define=API_BASE_URL=http://<host-ip>:8080
```

There's no self-serve sign-up. Create a school and its first correspondent login
with the backend's seed tool (see the root README), or use the OTP tab to sign in
as a parent whose guardian record already exists.

## Architecture

```
lib/
  core/
    api/            Thin REST client for /api/v1
    auth/           Login, OTP, session state, the SESSION_REVOKED wipe path
    db/              Drift + SQLCipher local database, one file per school
    connectivity/    Online/offline stream, drives sync triggers
  features/
    login/
    school_picker/   Multi-school staff switch schools here (PRD 3.2.1)
    home/            Academic year/class/section picker -> attendance
    attendance/      The offline-first marking screen and its repository
    parent/          Child selector, attendance/dues summary, notices inbox
```

The attendance module (`features/attendance/`) is the safety-critical piece: local-
first writes, an outbox queue, and a sync engine implementing the client side of the
exact conflict-resolution protocol the backend enforces in
`backend/internal/attendance/conflict.go`. See the doc comments in
`attendance_repository.dart` and `db/connection.dart` before changing either.

## Testing

```
flutter analyze
flutter test
```

`test/attendance_repository_test.dart` exercises the local-only write path against
an in-memory database -- no device, no SQLCipher, no network needed. It checks
specifically that `base_revision` and `local_counter` are captured correctly, since
a bug there would silently break the server-side conflict resolution in a way no
amount of backend-only testing could catch.

## A dependency note worth reading before touching `pubspec.yaml`

`drift`/`drift_dev` are pinned to the 2.31 line and `path_provider_foundation` is
overridden to a pre-hooks release. This isn't arbitrary: the current `sqlite3` 3.x
line bundles native SQLite via Dart's newer native-assets "hooks" build system,
which `build_runner`'s script compiler doesn't support yet -- even a transitive,
iOS/macOS-only dependency pulling in a hook breaks `dart run build_runner build`
entirely. See the comments in `pubspec.yaml` for the specifics. Also: never depend
on `sqlite3_flutter_libs` and `sqlcipher_flutter_libs` at the same time -- both
bundle a plugin class under the same package name, which fails Android's dex-merge
step. Only `sqlcipher_flutter_libs` is needed (PRD 6.4 requires the local database
always be encrypted, and SQLCipher's ABI is compatible with plain sqlite3).
