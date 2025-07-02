#!/bin/bash

if ! command -v freeze &> /dev/null; then
    echo "freeze not found. Please install freeze to capture images."
    echo "https://github.com/charmbracelet/freeze/"
    exit 1
fi

defaultStyles=("ascii" "auto" "dark" "dracula" "light" "notty" "pink")

for style in "${defaultStyles[@]}"; do
    echo "Generating screenshot for ${style}"
    # take screenshot
    if [[ $style == *"light"* ]]; then
        # Provide a light background to images
        freeze  -x "go run ./examples/artichokes ${style}" -b "#FAFAFA" -o "./styles/gallery/${style}.png"
    else
        freeze  -x "go run ./examples/artichokes ${style}" -o "./styles/gallery/${style}.png"
    fi

    # optimize filesize
    pngcrush -ow "./styles/gallery/$style.png"
done
