#!/bin/sh
# Nightly pg_dump for a single-VM deploy (PLAN.md Phase 10). No cron
# daemon: this is a personal, single-database backup, not a fleet — an
# infinite loop that sleeps until the next 03:00 UTC and dumps once a
# night is the whole job, and it's auditable in five lines instead of
# depending on a second scheduling mechanism inside the container.
set -eu

BACKUP_DIR=/backups
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"

# Pure arithmetic on H:M:S rather than `date -d "<string>"` — this image
# is Alpine, whose BusyBox `date` doesn't parse arbitrary date strings the
# way GNU date does, so this avoids depending on a coreutils flag that
# isn't there.
seconds_until_3am_utc() {
	h=$(date -u +%H); m=$(date -u +%M); s=$(date -u +%S)
	# 10# forces base-10 — a leading zero (e.g. "08") would otherwise be
	# read as an invalid octal literal by POSIX shell arithmetic.
	now_secs=$(( (10#$h * 3600) + (10#$m * 60) + 10#$s ))
	target_secs=$((3 * 3600))
	if [ "$now_secs" -lt "$target_secs" ]; then
		echo $((target_secs - now_secs))
	else
		echo $((86400 - now_secs + target_secs))
	fi
}

mkdir -p "$BACKUP_DIR"

while true; do
	sleep "$(seconds_until_3am_utc)"

	stamp=$(date -u +%Y%m%dT%H%M%SZ)
	dest="$BACKUP_DIR/pricewatch-${stamp}.dump"
	echo "backup: starting pg_dump -> $dest"

	if pg_dump --no-owner --format=custom -f "$dest"; then
		echo "backup: wrote $dest ($(du -h "$dest" | cut -f1))"
	else
		echo "backup: pg_dump failed, leaving previous backups untouched" >&2
		rm -f "$dest"
	fi

	find "$BACKUP_DIR" -name 'pricewatch-*.dump' -mtime "+${RETENTION_DAYS}" -delete
done
