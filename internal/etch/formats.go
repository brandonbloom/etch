package etch

func evalStructuredBytes(path, format, part, selector, verb, rawValue string, valueMode ValueMode, before []byte) ([]byte, bool, error) {
	if format == "" {
		format = "infer"
	}
	switch {
	case part == "frontmatter" || format == "frontmatter":
		value, err := parseStructuredValue(rawValue, valueMode)
		if err != nil {
			return nil, false, err
		}
		return evalFrontmatter(path, selector, verb, value, before)
	case format == "json" || (format == "infer" && isJSONPath(path)):
		out, changed, err := evalJSON(selector, verb, rawValue, valueMode, before)
		if err != nil {
			return nil, false, pathJSONInputParseError(path, err)
		}
		return out, changed, nil
	case format == "yaml" || (format == "infer" && isYAMLPath(path)):
		value, err := parseStructuredValue(rawValue, valueMode)
		if err != nil {
			return nil, false, err
		}
		return evalYAML(selector, verb, value, before)
	default:
		return nil, false, failf("cannot infer structured format for %s", path)
	}
}
