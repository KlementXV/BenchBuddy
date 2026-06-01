#!/usr/bin/env bash
# BenchBuddy interactive setup — builds the complete benchbuddy run command.
set -euo pipefail

# ── colours ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BOLD='\033[1m'; RESET='\033[0m'

# All UI helpers write to stderr so they remain visible when the function is
# called inside $(...) command substitution (which captures stdout).
info()  { echo -e "${GREEN}✔${RESET} $*" >&2; }
warn()  { echo -e "${YELLOW}⚠${RESET}  $*" >&2; }
error() { echo -e "${RED}✗${RESET} $*" >&2; }
ask()   { echo -e "${BOLD}${1}${RESET}" >&2; }

hr() { echo "────────────────────────────────────────────────────────────" >&2; }

# ── helpers ──────────────────────────────────────────────────────────────────
# prompt writes the result to stdout; the visible prompt of `read -rp` goes to
# stderr (bash default), so it stays visible even inside $(...).
prompt() {          # prompt <question> <default>
  local default="${2:-}" val
  if [[ -n "$default" ]]; then
    read -rp "  → [${default}]: " val
    echo "${val:-$default}"
  else
    read -rp "  → : " val
    echo "$val"
  fi
}

prompt_yn() {       # prompt_yn <question> <default y|n>
  local default="${2:-n}" val
  local hint="y/N"; [[ "$default" == "y" ]] && hint="Y/n"
  read -rp "  → [${hint}]: " val
  val="${val:-$default}"
  [[ "$val" =~ ^[Yy]$ ]]
}

choose() {          # choose <question> <opt1> <opt2> ...
  local question="$1"; shift
  local opts=("$@") val
  ask "$question"
  for i in "${!opts[@]}"; do
    echo "  $((i+1))) ${opts[$i]}" >&2
  done
  while true; do
    read -rp "  → [1]: " val
    val="${val:-1}"
    if [[ "$val" =~ ^[0-9]+$ ]] && (( val >= 1 && val <= ${#opts[@]} )); then
      echo "${opts[$((val-1))]}"; return
    fi
    warn "Please enter a number between 1 and ${#opts[@]}"
  done
}

# ── accumulate flags ─────────────────────────────────────────────────────────
FLAGS=()
add() { FLAGS+=("$@"); }

echo ""
echo -e "${BOLD}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}║        BenchBuddy — Interactive Setup    ║${RESET}"
echo -e "${BOLD}╚══════════════════════════════════════════╝${RESET}"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
hr; echo -e "${BOLD}1. Kubeconfig${RESET}"; hr

DEFAULT_KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"
ask "Kubeconfig path"
KUBECONFIG_PATH="$(prompt "" "$DEFAULT_KUBECONFIG")"

if [[ ! -f "$KUBECONFIG_PATH" ]]; then
  error "File not found: $KUBECONFIG_PATH"
  exit 1
fi
info "Found: $KUBECONFIG_PATH"
[[ "$KUBECONFIG_PATH" != "$DEFAULT_KUBECONFIG" ]] && add "--kubeconfig" "$KUBECONFIG_PATH"

# list available contexts
CONTEXTS=()
while IFS= read -r ctx; do CONTEXTS+=("$ctx"); done \
  < <(KUBECONFIG="$KUBECONFIG_PATH" kubectl config get-contexts -o name 2>/dev/null || true)

if [[ ${#CONTEXTS[@]} -eq 0 ]]; then
  error "No contexts found in $KUBECONFIG_PATH"
  exit 1
fi

CURRENT_CTX="$(KUBECONFIG="$KUBECONFIG_PATH" kubectl config current-context 2>/dev/null || echo "")"

if [[ ${#CONTEXTS[@]} -eq 1 ]]; then
  CTX="${CONTEXTS[0]}"
  info "Single context: $CTX"
else
  ask "Kubernetes context (current: ${CURRENT_CTX:-none})"
  for i in "${!CONTEXTS[@]}"; do
    marker=""; [[ "${CONTEXTS[$i]}" == "$CURRENT_CTX" ]] && marker=" ${GREEN}(current)${RESET}"
    echo -e "  $((i+1))) ${CONTEXTS[$i]}${marker}"
  done
  while true; do
    read -rp "  → [current]: " val
    if [[ -z "$val" && -n "$CURRENT_CTX" ]]; then CTX="$CURRENT_CTX"; break; fi
    if [[ "$val" =~ ^[0-9]+$ ]] && (( val >= 1 && val <= ${#CONTEXTS[@]} )); then
      CTX="${CONTEXTS[$((val-1))]}"; break
    fi
    warn "Enter a number between 1 and ${#CONTEXTS[@]}, or press Enter for current"
  done
fi
info "Context: $CTX"
[[ "$CTX" != "$CURRENT_CTX" ]] && add "--context" "$CTX"

# verify connectivity
echo "  Checking cluster connectivity..."
if ! KUBECONFIG="$KUBECONFIG_PATH" kubectl --context="$CTX" cluster-info --request-timeout=5s &>/dev/null; then
  warn "Cannot reach cluster — continuing anyway (you can still copy the command)"
else
  info "Cluster reachable"
fi

# ─────────────────────────────────────────────────────────────────────────────
hr; echo -e "${BOLD}2. Namespace${RESET}"; hr

ask "Target namespace (must already exist)"
NAMESPACE="$(prompt "" "benchbuddy")"

if KUBECONFIG="$KUBECONFIG_PATH" kubectl --context="$CTX" get namespace "$NAMESPACE" &>/dev/null 2>&1; then
  info "Namespace '$NAMESPACE' exists"
else
  warn "Namespace '$NAMESPACE' not found"
  ask "Create it now?"
  if prompt_yn "" "y"; then
    KUBECONFIG="$KUBECONFIG_PATH" kubectl --context="$CTX" create namespace "$NAMESPACE"
    info "Created namespace '$NAMESPACE'"
  else
    warn "Namespace must exist before running benchmarks"
  fi
fi
add "--namespace" "$NAMESPACE"

# ─────────────────────────────────────────────────────────────────────────────
hr; echo -e "${BOLD}3. Profile${RESET}"; hr

PROFILE="$(choose "Execution profile" \
  "quick    (~2 min)  — fast smoke test" \
  "standard (~10 min) — regular validation" \
  "deep     (~30 min) — pre-production thoroughness")"
PROFILE="${PROFILE%% *}"
info "Profile: $PROFILE"
add "--profile" "$PROFILE"

# ─────────────────────────────────────────────────────────────────────────────
hr; echo -e "${BOLD}4. Airgap${RESET}"; hr

ask "Is this an airgap environment (no internet access from nodes)?"
if prompt_yn "" "n"; then
  AIRGAP=true

  ask "Private registry base (e.g. registry.corp.internal/benchbuddy)"
  REGISTRY="$(prompt "" "")"
  if [[ -n "$REGISTRY" ]]; then
    add "--registry" "$REGISTRY"
    info "Registry: $REGISTRY"
  fi

  ask "Runner image tag already mirrored (e.g. v0.1.0 — leave blank to use default)"
  RUNNER_TAG="$(prompt "" "")"
  if [[ -n "$RUNNER_TAG" ]]; then
    add "--runner-image" "runner:${RUNNER_TAG}"
    info "Runner image tag: $RUNNER_TAG"
  fi

  ask "Runner image digest (sha256:... — leave blank to skip)"
  RUNNER_DIGEST="$(prompt "" "")"
  if [[ -n "$RUNNER_DIGEST" ]]; then
    add "--runner-digest" "$RUNNER_DIGEST"
    info "Digest pinned"
  fi

  ask "Pause image full ref mirrored in your registry (e.g. registry.corp.internal/pause:3.9 — leave blank to keep default registry.k8s.io/pause:3.9)"
  PAUSE_IMAGE="$(prompt "" "")"
  if [[ -n "$PAUSE_IMAGE" ]]; then
    add "--pause-image" "$PAUSE_IMAGE"
    info "Pause image: $PAUSE_IMAGE"
  fi

  ask "ImagePullSecret name in namespace '$NAMESPACE' (leave blank if none)"
  PULL_SECRET="$(prompt "" "")"
  if [[ -n "$PULL_SECRET" ]]; then
    add "--image-pull-secret" "$PULL_SECRET"
    info "Pull secret: $PULL_SECRET"
  fi

  add "--image-pull-policy" "IfNotPresent"
else
  AIRGAP=false
  info "Standard (online) environment"
fi

# ─────────────────────────────────────────────────────────────────────────────
hr; echo -e "${BOLD}5. Output formats${RESET}"; hr

ask "Save JSON output? (leave blank to skip)"
read -rp "  → path [e.g. report.json]: " JSON_PATH
[[ -n "$JSON_PATH" ]] && add "--output" "json=${JSON_PATH}" && info "JSON → $JSON_PATH"

ask "Save Markdown output? (leave blank to skip)"
read -rp "  → path [e.g. report.md]: " MD_PATH
[[ -n "$MD_PATH" ]] && add "--output" "md=${MD_PATH}" && info "Markdown → $MD_PATH"

# ─────────────────────────────────────────────────────────────────────────────
hr; echo -e "${BOLD}6. Advanced options${RESET}"; hr

ask "Exclude specific benches? (comma-separated: network,storage,dns,api,pod — leave blank for none)"
read -rp "  → : " EXCLUDE_BENCH
if [[ -n "$EXCLUDE_BENCH" ]]; then
  add "--exclude-bench" "$EXCLUDE_BENCH"
  info "Excluded benches: $EXCLUDE_BENCH"
fi

ask "Exclude specific nodes? (comma-separated node names — leave blank for none)"
read -rp "  → : " EXCLUDE_NODE
if [[ -n "$EXCLUDE_NODE" ]]; then
  add "--exclude-node" "$EXCLUDE_NODE"
  info "Excluded nodes: $EXCLUDE_NODE"
fi

# Try to list available StorageClasses to help the user pick
SC_LIST=""
if KUBECONFIG="$KUBECONFIG_PATH" kubectl --context="$CTX" get storageclass -o name --request-timeout=5s &>/dev/null; then
  SC_LIST="$(KUBECONFIG="$KUBECONFIG_PATH" kubectl --context="$CTX" get storageclass -o name 2>/dev/null | sed 's|storageclass.storage.k8s.io/||' | paste -sd, -)"
  [[ -n "$SC_LIST" ]] && echo -e "  ${YELLOW}ℹ${RESET}  Available StorageClasses in cluster: $SC_LIST" >&2
fi

ask "Test ONLY specific StorageClasses? (comma-separated whitelist — leave blank to test all)"
read -rp "  → : " INCLUDE_SC
if [[ -n "$INCLUDE_SC" ]]; then
  add "--include-storageclass" "$INCLUDE_SC"
  info "Testing only: $INCLUDE_SC"
fi

ask "Exclude specific StorageClasses? (comma-separated — leave blank for none)"
read -rp "  → : " EXCLUDE_SC
if [[ -n "$EXCLUDE_SC" ]]; then
  add "--exclude-storageclass" "$EXCLUDE_SC"
  info "Excluded StorageClasses: $EXCLUDE_SC"
fi

ask "Skip the confirmation prompt before running? (--yes)"
if prompt_yn "" "n"; then
  add "--yes"
  info "--yes flag added"
fi

ask "Keep resources after run? (--keep, useful for debugging)"
if prompt_yn "" "n"; then
  add "--keep"
  info "--keep flag added"
fi

# ─────────────────────────────────────────────────────────────────────────────
hr; echo -e "${BOLD}7. RBAC check${RESET}"; hr

echo "  Verifying required RBAC permissions..."
RBAC_OK=true
MISSING=()

check_verb() {   # check_verb <verb> <resource> [<group>]
  local verb="$1" resource="$2" group="${3:-}"
  local args=(--context="$CTX" -n "$NAMESPACE" auth can-i "$verb" "$resource")
  [[ -n "$group" ]] && args+=(--subresource="" )
  if ! KUBECONFIG="$KUBECONFIG_PATH" kubectl "${args[@]}" &>/dev/null 2>&1; then
    MISSING+=("$verb $resource")
    RBAC_OK=false
  fi
}

check_verb create pods
check_verb get    pods
check_verb list   pods
check_verb delete pods
check_verb create persistentvolumeclaims
check_verb delete persistentvolumeclaims
check_verb list   nodes

if $RBAC_OK; then
  info "All checked RBAC permissions present"
else
  warn "Missing permissions (benchbuddy will report these too):"
  for m in "${MISSING[@]}"; do echo "    - $m"; done
  warn "Apply the RBAC manifests from the README before running"
fi

# ─────────────────────────────────────────────────────────────────────────────
hr
echo ""
echo -e "${BOLD}Your benchbuddy command:${RESET}"
echo ""

CMD="benchbuddy run"
for flag in "${FLAGS[@]}"; do
  # wrap values with spaces in quotes
  if [[ "$flag" == --* ]]; then
    CMD+=" $flag"
  else
    if [[ "$flag" == *" "* ]]; then
      CMD+=" \"$flag\""
    else
      CMD+=" $flag"
    fi
  fi
done

echo -e "  ${GREEN}${CMD}${RESET}"
echo ""

# write to file for easy copy
OUTFILE="$(pwd)/benchbuddy-run.sh"
{
  echo "#!/usr/bin/env bash"
  echo "# Generated by benchbuddy setup.sh on $(date -u '+%Y-%m-%d %H:%M UTC')"
  echo ""
  echo "$CMD"
} > "$OUTFILE"
chmod +x "$OUTFILE"

info "Command saved to: $OUTFILE"
echo ""
hr

ask "Run it now?"
if prompt_yn "" "n"; then
  echo ""
  eval "$CMD"
else
  echo ""
  echo "  Run it later with:"
  echo -e "  ${BOLD}${CMD}${RESET}"
  echo "  or: bash $OUTFILE"
fi
echo ""
