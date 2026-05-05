package parsers

import "github.com/trickyearlobe/log-analysis-mcp/internal/types"

// registeredParsers returns parsers in priority order for tiebreaking.
// Priority: JSON > Erlang SASL > Habitat Sup > Syslog RFC 5424 > journalctl ISO > Syslog RFC 3164 > Apache Combined > Apache Common.
func registeredParsers() []Parser {
	return []Parser{
		NewJSONParser(),
		NewErlangSASLParser(),
		NewHabitatSupParser(),
		NewSyslogRFC5424Parser(),
		NewJournalISOParser(),
		NewSyslogRFC3164Parser(),
		NewApacheCombinedParser(),
		NewApacheCommonParser(),
	}
}

// AutoDetect scores every registered parser against the provided sample lines
// and returns the best match. Parsers are evaluated in priority order so that
// ties are broken deterministically (earlier in the list wins).
func AutoDetect(lines []string) types.FormatDetectionResult {
	if len(lines) == 0 {
		return types.FormatDetectionResult{
			Format:     types.LogFormatUnknown,
			Confidence: 0,
			SampleSize: 0,
		}
	}

	// Filter out empty lines — spec says "first 10 non-empty lines".
	nonEmpty := make([]string, 0, len(lines))
	for _, l := range lines {
		if l != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}
	if len(nonEmpty) == 0 {
		return types.FormatDetectionResult{
			Format:     types.LogFormatUnknown,
			Confidence: 0,
			SampleSize: 0,
		}
	}

	var (
		bestFormat     types.LogFormat
		bestConfidence float64
		bestSuccessful int
	)

	for _, p := range registeredParsers() {
		successful := 0
		for _, line := range nonEmpty {
			if p.Parse(line) != nil {
				successful++
			}
		}
		score := float64(successful) / float64(len(nonEmpty))

		// Strictly greater — earlier parsers win ties due to iteration order.
		if score > bestConfidence {
			bestConfidence = score
			bestFormat = types.LogFormat(p.Name())
			bestSuccessful = successful
		}
	}

	if bestConfidence < 0.5 {
		return types.FormatDetectionResult{
			Format:           types.LogFormatUnknown,
			Confidence:       bestConfidence,
			SampleSize:       len(nonEmpty),
			SuccessfulParses: bestSuccessful,
		}
	}

	return types.FormatDetectionResult{
		Format:           bestFormat,
		Confidence:       bestConfidence,
		SampleSize:       len(nonEmpty),
		SuccessfulParses: bestSuccessful,
	}
}

// AutoDetectWithHint handles the format_hint parameter. If hint is non-empty
// and not "auto", it returns the corresponding parser directly. Otherwise it
// delegates to AutoDetect.
func AutoDetectWithHint(lines []string, hint string) (types.FormatDetectionResult, Parser) {
	if hint != "" && hint != "auto" {
		for _, p := range registeredParsers() {
			if p.Name() == hint {
				return types.FormatDetectionResult{
					Format:     types.LogFormat(hint),
					Confidence: 1.0,
					SampleSize: len(lines),
				}, p
			}
		}
		// Hint didn't match any known parser — fall through to auto-detect.
	}

	result := AutoDetect(lines)
	if result.Format == types.LogFormatUnknown {
		return result, nil
	}
	for _, p := range registeredParsers() {
		if p.Name() == string(result.Format) {
			return result, p
		}
	}
	return result, nil
}
