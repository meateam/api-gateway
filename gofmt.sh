#!/bin/bash
gofiles=$(find . -name "*.go")
[ -z "$gofiles" ] && exit 0

unformatted=$(gofmt -l $gofiles)
[ -z "$unformatted" ] && exit 0

echo >&2 "Formatting go files..."
for fn in $unformatted; do
  echo "   gofmt -w $PWD/$fn"
        gofmt -w $PWD/$fn
done

exit 1