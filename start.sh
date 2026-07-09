#!/bin/ash

set -e

if [ $# -lt 1 ]; then
    echo "Usage: $0 <CMD> [CMD_ARGS...]"
    exit 1
fi

to_lower_dash() { echo "$1" | tr 'A-Z' 'a-z' | tr '-' '_'; }

CMD="$1"
ENV_PREFIX="$(to_lower_dash "$CMD")_"
shift

ARGS=""
# Collect matching environment variables
while IFS='=' read -r name value; do
    normalized_name="$(to_lower_dash "$name")"
    case "$normalized_name" in
        ${ENV_PREFIX}*)
            arg_name="${normalized_name#$ENV_PREFIX}"
            ARGS="$ARGS --$arg_name $value"
            ;;
    esac
done <<EOF
$(env)
EOF

# Run the command with arguments
exec "$CMD" $ARGS "$@"
