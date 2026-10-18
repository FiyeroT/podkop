package engine

import (
	"strings"

	"github.com/sagernet/sing-box/common/sharelink"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
)

// A proxy link of the settings becomes an outbound the way the command "sing-box tools
// decode-link" makes it: decoded by common/sharelink, and built by sing-box with the
// configuration it goes into. A link this sing-box cannot use is refused with its reason
// before the configuration runs.

// DecodeLink returns the outbound of a link as a JSON object and the warnings about what
// the link asks for and the outbound cannot do. The text is what the settings hold: one
// link, with white space and empty lines around it allowed. The messages do not repeat
// the link, which is a secret.
//
// The outbound is not built here: the check of the whole configuration builds it, one
// build for all the links instead of one a link and a plan, and
// CheckLink names the link of a configuration that sing-box refused.
func DecodeLink(text string) ([]byte, []string, error) {
	_, content, warnings, err := decodeLink(text)
	if err != nil {
		return nil, nil, err
	}
	var lines []string
	for _, warning := range warnings {
		lines = append(lines, sharelink.OneLine(warning))
	}
	return content, lines, nil
}

// CheckLink builds the outbound of a link alone, as the only one of a configuration: the
// error tells why this sing-box cannot use the link, without what the link keeps secret.
func CheckLink(text string) error {
	outbound, content, _, err := decodeLink(text)
	if err != nil {
		return err
	}
	_, err = CheckOutbound(content)
	if err != nil {
		return E.New(sharelink.PlainMessage(err.Error(), append(outbound.Secrets(), outbound.Unplain()...)))
	}
	return nil
}

func decodeLink(text string) (*sharelink.Outbound, []byte, []string, error) {
	var link string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if line == "" {
			continue
		}
		if link != "" {
			return nil, nil, nil, E.New("more than one link")
		}
		link = line
	}
	if link == "" {
		return nil, nil, nil, E.New("no link")
	}
	outbound, warnings, err := sharelink.Decode(link)
	if err != nil {
		return nil, nil, nil, E.New(sharelink.PlainMessage(err.Error(), nil))
	}
	content, err := json.Marshal(outbound)
	if err != nil {
		return nil, nil, nil, E.Cause(err, "encode the outbound")
	}
	return outbound, content, warnings, nil
}
