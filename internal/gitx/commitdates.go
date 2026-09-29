package gitx

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// commitDateLayout is CommitDate's canonical output layout (log.go): an
// ISO-8601 instant in the committer's own offset, the offset always numeric
// ("+00:00", never "Z"), so the batch read below and the single read agree
// byte for byte (TestCommitDates_MatchesCommitDate pins the two together).
const commitDateLayout = "2006-01-02T15:04:05-07:00"

// CommitDates returns the committer date of every rev in revs that names a
// commit in dir, keyed by the rev exactly as given, in CommitDate's
// canonical form — read in ONE git invocation (`git cat-file --batch`),
// however many revs are asked (spec/index-data co-1 and the index-data
// review's budget finding: the directory index reads every entry's date in
// one pass per walk, never one `git log -1` process per entry).
//
// A rev that does not name a commit (unknown, ambiguous, or naming a blob
// or tree) is simply absent from the map — never an error and never a zero
// date — so a caller can disclose that one entry while every other rev
// still answers. Each rev is peeled with ^{commit}, so an annotated tag
// answers its commit's committer date, never the tagger date. A rev that is
// empty or carries a line break is refused as absent: the batch protocol is
// line-oriented, and such a rev would otherwise inject a second query.
// Duplicate revs are asked once. With nothing askable the answer is an
// empty map and no git process runs at all.
//
// The error is reserved for the one invocation itself failing — dir not a
// repository, git unavailable, a cancelled ctx, or batch output this
// function cannot parse — never for one rev's absence.
func CommitDates(ctx context.Context, dir string, revs []string) (map[string]string, error) {
	dates := make(map[string]string, len(revs))
	queries := make([]string, 0, len(revs))
	asked := make(map[string]bool, len(revs))
	for _, rev := range revs {
		if rev == "" || strings.ContainsAny(rev, "\r\n") || asked[rev] {
			continue
		}
		asked[rev] = true
		queries = append(queries, rev)
	}
	if len(queries) == 0 {
		return dates, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("gitx: CommitDates: %w", err)
	}

	var stdin bytes.Buffer
	for _, rev := range queries {
		stdin.WriteString(rev)
		stdin.WriteString("^{commit}\n")
	}
	out, err := runStdin(ctx, dir, nil, stdin.Bytes(), "cat-file", "--batch")
	if err != nil {
		return nil, fmt.Errorf("gitx: CommitDates: %w", err)
	}

	answers := bufio.NewReader(bytes.NewReader(out))
	for _, rev := range queries {
		body, found, err := readBatchAnswer(answers)
		if err != nil {
			return nil, fmt.Errorf("gitx: CommitDates: reading the answer for %q: %w", rev, err)
		}
		if !found {
			continue
		}
		if date, ok := committerDate(body); ok {
			dates[rev] = date
		}
	}
	return dates, nil
}

// readBatchAnswer reads one `git cat-file --batch` answer: either a
// "<query> missing" / "<query> ambiguous" line (found false), or a
// "<oid> commit <size>" header followed by exactly size content bytes and
// one newline (found true, with the content). Any other shape is an error:
// the stream can no longer be kept in step with the queries.
func readBatchAnswer(r *bufio.Reader) (body []byte, found bool, err error) {
	header, err := r.ReadString('\n')
	if err != nil {
		return nil, false, fmt.Errorf("batch header: %w", err)
	}
	header = strings.TrimSuffix(header, "\n")
	if strings.HasSuffix(header, " missing") || strings.HasSuffix(header, " ambiguous") {
		return nil, false, nil
	}
	fields := strings.Fields(header)
	if len(fields) != 3 || fields[1] != "commit" {
		return nil, false, fmt.Errorf("unexpected batch header %q", header)
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil || size < 0 {
		return nil, false, fmt.Errorf("unexpected batch object size in %q", header)
	}
	body = make([]byte, size+1) // the content plus its trailing newline
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, false, fmt.Errorf("batch content: %w", err)
	}
	if body[size] != '\n' {
		return nil, false, fmt.Errorf("batch content for %q is not newline-terminated", header)
	}
	return body[:size], true, nil
}

// committerDate reads a raw commit object's committer timestamp —
// "committer <name> <<email>> <unix-seconds> <+hhmm>" in the header block
// before the first blank line — as CommitDate's canonical form in the
// committer's own offset. ok is false when no well-formed committer line is
// present: the caller treats that rev as unreadable, never as a zero date.
func committerDate(body []byte) (date string, ok bool) {
	for _, line := range strings.Split(string(body), "\n") {
		if line == "" {
			return "", false // end of the header block
		}
		if !strings.HasPrefix(line, "committer ") {
			continue
		}
		end := strings.LastIndexByte(line, '>')
		if end < 0 {
			return "", false
		}
		stamp := strings.Fields(line[end+1:])
		if len(stamp) != 2 {
			return "", false
		}
		secs, err := strconv.ParseInt(stamp[0], 10, 64)
		if err != nil {
			return "", false
		}
		offset, ok := parseGitOffset(stamp[1])
		if !ok {
			return "", false
		}
		return time.Unix(secs, 0).In(time.FixedZone("", offset)).Format(commitDateLayout), true
	}
	return "", false
}

// parseGitOffset parses git's "+hhmm"/"-hhmm" offset into seconds east of
// UTC. Exactly four ASCII digits follow the one sign — strconv.Atoi alone
// would also accept a second sign inside the hour or minute field.
func parseGitOffset(tz string) (int, bool) {
	if len(tz) != 5 || (tz[0] != '+' && tz[0] != '-') {
		return 0, false
	}
	for i := 1; i < len(tz); i++ {
		if tz[i] < '0' || tz[i] > '9' {
			return 0, false
		}
	}
	hours := int(tz[1]-'0')*10 + int(tz[2]-'0')
	minutes := int(tz[3]-'0')*10 + int(tz[4]-'0')
	if minutes > 59 {
		return 0, false
	}
	seconds := hours*3600 + minutes*60
	if tz[0] == '-' {
		seconds = -seconds
	}
	return seconds, true
}
