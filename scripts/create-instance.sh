#!/usr/bin/env bash

set -euo pipefail

usage() {
  echo "Usage: $0 <name> <main-dir>" >&2
  echo "Example: $0 gym2 ~/.local/share/owlcms-controlpanel" >&2
}

if [[ $# -ne 2 ]]; then
  usage
  exit 1
fi

name="$1"
main_dir="$2"

if [[ ! "$name" =~ ^[A-Za-z0-9-]+$ ]]; then
  echo "Instance name must contain only letters, numbers, and hyphens." >&2
  exit 1
fi

main_dir="${main_dir/#\~/$HOME}"
main_dir="$(realpath -m "$main_dir")"
share_dir="$(dirname "$main_dir")"

owlcms_dir="$share_dir/owlcms-$name"
tracker_dir="$share_dir/owlcms-tracker-$name"
controlpanel_dir="$share_dir/owlcms-controlpanel-$name"
scripts_dir="$main_dir/scripts"
launcher_path="$scripts_dir/controlpanel-$name"

mkdir -p "$main_dir" "$scripts_dir" "$owlcms_dir" "$tracker_dir" "$controlpanel_dir"

if [[ ! -f "$owlcms_dir/env.properties" ]]; then
  cat > "$owlcms_dir/env.properties" <<'EOF'
OWLCMS_PORT=8081
TEMURIN_VERSION=jdk-25
EOF
fi

if [[ ! -f "$tracker_dir/env.properties" ]]; then
  cat > "$tracker_dir/env.properties" <<'EOF'
TRACKER_PORT=8097
EOF
fi

cat > "$launcher_path" <<EOF
#!/usr/bin/env bash
set -euo pipefail

export RUNTIME_DIR="$main_dir"
export OWLCMS_INSTALLDIR="$owlcms_dir"
export TRACKER_INSTALLDIR="$tracker_dir"
export CONTROLPANEL_INSTALLDIR="$controlpanel_dir"
export CONTROLPANEL_INSTANCE="$name"

exec controlpanel "\$@"
EOF

chmod +x "$launcher_path"

echo "Created instance '$name'"
echo "  runtime dir:       $main_dir"
echo "  owlcms dir:        $owlcms_dir"
echo "  tracker dir:       $tracker_dir"
echo "  control panel dir: $controlpanel_dir"
echo "  launcher:          $launcher_path"