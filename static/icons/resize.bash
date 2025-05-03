#!/usr/bin/env bash

set -euo pipefail

WEBP_QUALITY=90

if ! command -v magick &>/dev/null; then
	echo "Error: ImageMagick (magick command) not found."
	exit 1
fi

# Check if an input file was provided.
if [[ -z "${1:-}" ]]; then
	echo "Usage: $0 <input_image_file>"
	exit 1
fi

INPUT_FILE="$1"

if [[ ! -f "$INPUT_FILE" ]]; then
	echo "Error: Input file $INPUT_FILE not found."
	exit 1
fi

SIZES=("179" "191" "35")

for SIZE in "${SIZES[@]}"; do
	OUTPUT_FILE="${SIZE}x${SIZE}.webp"

	# Calculate center coordinates for the circle mask (relative to the final square).
	CENTER_X=$(($SIZE / 2))
	CENTER_Y=$(($SIZE / 2))
	# Point on perimeter for circle drawing (top-center).
	PERIM_Y=0

	magick "$INPUT_FILE" \
		-resize "${SIZE}x${SIZE}^" \
		-gravity North \
		-extent "${SIZE}x${SIZE}" \
		\( +clone -alpha transparent -fill white -draw "circle $CENTER_X,$CENTER_Y $CENTER_X,$PERIM_Y" \) \
		-compose CopyOpacity \
		-composite \
		-quality "$WEBP_QUALITY" \
		"$OUTPUT_FILE"
done
