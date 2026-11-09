#!/usr/bin/env bash
# Create an admin-console user, or reset an existing user's password.
#
#   scripts/create-user.sh --email you@example.com [--role owner|admin|reviewer] [--name "Your Name"]
#   scripts/create-user.sh --email you@example.com --reset          # reset password (revokes sessions)
#
# The password is read from ADMIN_PASSWORD, or prompted for (hidden, typed twice). It must be at
# least 12 characters. It is never written to disk or to shell history by this script.
#
# Runs inside the Docker Compose "api" container when it is up; otherwise uses `go run ./apps/api`
# with the settings in .env (DATABASE_URL etc.). After the first owner exists, owners can create
# and manage further users in the admin console: Admin → Users.
set -euo pipefail
cd "$(dirname "$0")/.."

email="" role="owner" name="" reset=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --email) email="${2:-}"; shift 2 ;;
    --role) role="${2:-}"; shift 2 ;;
    --name) name="${2:-}"; shift 2 ;;
    --reset) reset=1; shift ;;
    -h|--help) sed -n '2,13p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown option: $1 (see --help)" >&2; exit 2 ;;
  esac
done

[[ -n "$email" ]] || { echo "--email is required (see --help)" >&2; exit 2; }
case "$role" in owner|admin|reviewer) ;; *) echo "--role must be owner, admin or reviewer" >&2; exit 2 ;; esac

password="${ADMIN_PASSWORD:-}"
if [[ -z "$password" ]]; then
  read -rsp "Password for $email (min 12 characters): " password; echo
  read -rsp "Repeat password: " again; echo
  [[ "$password" == "$again" ]] || { echo "passwords do not match" >&2; exit 1; }
fi
if (( ${#password} < 12 )); then
  echo "password must be at least 12 characters (got ${#password})" >&2
  exit 1
fi

if (( reset )); then
  args=(set-password --email "$email")
else
  args=(create-admin --email "$email" --role "$role" --name "$name")
fi

if docker compose ps --status running --services 2>/dev/null | grep -qx api; then
  docker compose exec -T -e ADMIN_PASSWORD="$password" api /api "${args[@]}"
else
  echo "api container not running; using go run with .env" >&2
  # Export KEY=VALUE lines literally (values such as MAIL_FROM contain spaces and <> and must not be
  # evaluated by the shell). Variables already set in the environment win.
  if [[ -f .env ]]; then
    while IFS= read -r line || [[ -n "$line" ]]; do
      [[ "$line" =~ ^[A-Za-z_][A-Za-z0-9_]*= ]] || continue
      key="${line%%=*}"
      [[ -n "${!key:-}" ]] || export "$key=${line#*=}"
    done < .env
  fi
  ADMIN_PASSWORD="$password" go run ./apps/api "${args[@]}"
fi
