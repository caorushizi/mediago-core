#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Mock Data Generator for mediago testing
#
# Generates HLS and DASH test data using ffmpeg, optionally
# uploads to rustfs via mc.
#
# Usage:
#   ./generate.sh              # generate locally
#   ./generate.sh --upload     # generate + upload to rustfs
#
# Configuration: source config.sh or set environment variables.
# See config.sh.example for available variables.
# ============================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DUMMY_VIDEO_DURATION=6

# --- Configuration ---
# Environment variables take precedence over config.sh values.
# Save any env overrides before sourcing config.sh.

_env_endpoint="${RUSTFS_ENDPOINT:-}"
_env_access_key="${RUSTFS_ACCESS_KEY:-}"
_env_secret_key="${RUSTFS_SECRET_KEY:-}"
_env_bucket="${RUSTFS_BUCKET:-}"
_env_tmp_dir="${MOCK_DATA_TMP_DIR:-}"
_env_source_url="${SOURCE_VIDEO_URL-__unset__}"

CONFIG_FILE="${SCRIPT_DIR}/config.sh"
if [[ -f "$CONFIG_FILE" ]]; then
    # shellcheck source=/dev/null
    source "$CONFIG_FILE"
fi

# Restore env overrides (env vars win over config.sh)
[[ -n "$_env_endpoint" ]]   && RUSTFS_ENDPOINT="$_env_endpoint"
[[ -n "$_env_access_key" ]] && RUSTFS_ACCESS_KEY="$_env_access_key"
[[ -n "$_env_secret_key" ]] && RUSTFS_SECRET_KEY="$_env_secret_key"
[[ -n "$_env_bucket" ]]     && RUSTFS_BUCKET="$_env_bucket"
[[ -n "$_env_tmp_dir" ]]    && MOCK_DATA_TMP_DIR="$_env_tmp_dir"
[[ "$_env_source_url" != "__unset__" ]] && SOURCE_VIDEO_URL="$_env_source_url"

RUSTFS_ENDPOINT="${RUSTFS_ENDPOINT:?Missing RUSTFS_ENDPOINT}"
RUSTFS_ACCESS_KEY="${RUSTFS_ACCESS_KEY:?Missing RUSTFS_ACCESS_KEY}"
RUSTFS_SECRET_KEY="${RUSTFS_SECRET_KEY:?Missing RUSTFS_SECRET_KEY}"
RUSTFS_BUCKET="${RUSTFS_BUCKET:-video-server}"
MOCK_DATA_TMP_DIR="${MOCK_DATA_TMP_DIR:-/tmp/mediago-mockdata}"
SOURCE_VIDEO_URL="${SOURCE_VIDEO_URL:-}"

# --- Helpers ---

log()  { echo "  $*" >&2; }
step() { echo -e "\n[$1] $2" >&2; }

# --- Source video ---

get_source_video() {
    local tmp_dir="$1"
    local source_video="${tmp_dir}/source_video.mp4"

    if [[ -n "$SOURCE_VIDEO_URL" ]]; then
        log "Downloading source video..."
        curl -fsSL -o "$source_video" "$SOURCE_VIDEO_URL"
        log "Downloaded to: $source_video"
    else
        log "Generating ${DUMMY_VIDEO_DURATION}s test video with testsrc..."
        ffmpeg -y \
            -f lavfi -i "testsrc=duration=${DUMMY_VIDEO_DURATION}:size=1280x720:rate=25" \
            -f lavfi -i "sine=frequency=440:duration=${DUMMY_VIDEO_DURATION}" \
            -c:v libx264 -preset ultrafast \
            -c:a aac -b:a 128k \
            -pix_fmt yuv420p \
            "$source_video" 2>/dev/null
        log "Generated: $source_video"
    fi

    echo "$source_video"
}

# --- HLS generation ---

generate_hls() {
    local source_video="$1"
    local output_dir="$2"
    local low_dir="${output_dir}/low"
    local high_dir="${output_dir}/high"

    mkdir -p "$low_dir" "$high_dir"

    log "Generating low quality HLS (640x360)..."
    ffmpeg -y -i "$source_video" \
        -c:v libx264 -preset ultrafast -b:v 800k \
        -c:a aac -b:a 96k \
        -hls_time 2 -hls_list_size 0 \
        -hls_segment_filename "${low_dir}/segment%d.ts" \
        "${low_dir}/index.m3u8" 2>/dev/null

    log "Generating high quality HLS (1280x720)..."
    ffmpeg -y -i "$source_video" \
        -c:v libx264 -preset ultrafast -b:v 1600k \
        -c:a aac -b:a 128k \
        -hls_time 2 -hls_list_size 0 \
        -hls_segment_filename "${high_dir}/segment%d.ts" \
        "${high_dir}/index.m3u8" 2>/dev/null

    # Master playlist
    cat > "${output_dir}/master.m3u8" << 'PLAYLIST'
#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=640x360,CODECS="avc1.640028,mp4a.40.2"
low/index.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=1600000,RESOLUTION=1280x720,CODECS="avc1.640028,mp4a.40.2"
high/index.m3u8
PLAYLIST

    local low_count high_count
    low_count=$(find "$low_dir" -name 'segment*.ts' | wc -l | tr -d ' ')
    high_count=$(find "$high_dir" -name 'segment*.ts' | wc -l | tr -d ' ')
    log "HLS generated: ${low_count} low + ${high_count} high segments"
}

# --- DASH generation ---

generate_dash() {
    local source_video="$1"
    local output_dir="$2"

    mkdir -p "$output_dir"

    log "Generating multi-bitrate DASH output..."
    ffmpeg -y -i "$source_video" \
        -map 0:v -map 0:a \
        -c:v:0 libx264 -preset ultrafast -b:v:0 800k -s:v:0 640x360 \
        -c:a:0 aac -b:a:0 96k \
        -map 0:v -map 0:a \
        -c:v:1 libx264 -preset ultrafast -b:v:1 1600k -s:v:1 1280x720 \
        -c:a:1 aac -b:a:1 128k \
        -f dash \
        -seg_duration 2 \
        -use_template 1 \
        -use_timeline 1 \
        -init_seg_name 'init-stream$RepresentationID$.m4s' \
        -media_seg_name 'chunk-stream$RepresentationID$-$Number%05d$.m4s' \
        -adaptation_sets "id=0,streams=v id=1,streams=a" \
        "${output_dir}/manifest.mpd" 2>/dev/null

    local init_count seg_count
    init_count=$(find "$output_dir" -name 'init-*.m4s' | wc -l | tr -d ' ')
    seg_count=$(find "$output_dir" -name 'chunk-*.m4s' | wc -l | tr -d ' ')
    log "DASH generated: ${init_count} init + ${seg_count} media segments"
}

# --- Upload ---

upload_to_rustfs() {
    local tmp_dir="$1"

    log "Setting up mc alias..."
    mc alias set rustfs "$RUSTFS_ENDPOINT" "$RUSTFS_ACCESS_KEY" "$RUSTFS_SECRET_KEY" >/dev/null

    log "Creating bucket: $RUSTFS_BUCKET"
    mc mb --ignore-existing "rustfs/${RUSTFS_BUCKET}" >/dev/null
    mc anonymous set download "rustfs/${RUSTFS_BUCKET}" >/dev/null

    for dir in hls dash; do
        if [[ -d "${tmp_dir}/${dir}" ]]; then
            log "Uploading ${dir}/..."
            mc cp --recursive "${tmp_dir}/${dir}" "rustfs/${RUSTFS_BUCKET}/${dir}/"
        fi
    done
}

# --- Main ---

main() {
    local upload=false
    if [[ "${1:-}" == "--upload" ]]; then
        upload=true
    fi

    echo "=================================================="
    echo "Mock Data Generator for mediago"
    echo "=================================================="
    echo "Endpoint: $RUSTFS_ENDPOINT"
    echo "Bucket:   $RUSTFS_BUCKET"
    echo "Tmp Dir:  $MOCK_DATA_TMP_DIR"
    if [[ -n "$SOURCE_VIDEO_URL" ]]; then
        echo "Source:   Real video (SOURCE_VIDEO_URL set)"
    else
        echo "Source:   Synthetic test video (ffmpeg testsrc)"
    fi

    # Clean and create tmp dir
    rm -rf "$MOCK_DATA_TMP_DIR"
    mkdir -p "$MOCK_DATA_TMP_DIR"

    step "1/4" "Preparing source video..."
    local source_video
    source_video=$(get_source_video "$MOCK_DATA_TMP_DIR")

    step "2/4" "Generating HLS mock data..."
    generate_hls "$source_video" "${MOCK_DATA_TMP_DIR}/hls"

    step "3/4" "Generating DASH mock data..."
    generate_dash "$source_video" "${MOCK_DATA_TMP_DIR}/dash"

    # Cleanup source video
    rm -f "$source_video"

    if [[ "$upload" == true ]]; then
        step "4/4" "Uploading to rustfs..."
        upload_to_rustfs "$MOCK_DATA_TMP_DIR"

        echo ""
        echo "=================================================="
        echo "Upload complete!"
        echo "=================================================="
        echo ""
        echo "HLS test URL:"
        echo "  ${RUSTFS_ENDPOINT}/${RUSTFS_BUCKET}/hls/master.m3u8"
        echo ""
        echo "DASH test URL:"
        echo "  ${RUSTFS_ENDPOINT}/${RUSTFS_BUCKET}/dash/manifest.mpd"
        echo ""
        echo "Uploaded files:"
        mc ls --recursive "rustfs/${RUSTFS_BUCKET}/"
    else
        echo ""
        echo "=================================================="
        echo "Mock data generated locally!"
        echo "=================================================="
        echo "Local files:"
        echo "  ${MOCK_DATA_TMP_DIR}/hls/"
        echo "  ${MOCK_DATA_TMP_DIR}/dash/"
        echo ""
        echo "To upload to rustfs, run:"
        echo "  $0 --upload"
    fi

    echo ""
    echo "Temporary files kept at: $MOCK_DATA_TMP_DIR"
}

main "$@"
