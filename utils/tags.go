package utils

import (
	"errors"

	"github.com/hymatrix/hymx/schema"
	goarSchema "github.com/permadao/goar/schema"
)

var (
	NonExtractableTags = map[string]string{
		"Data-Protocol": "Data-Protocol",
		"Variant":       "Variant",
		"From-Process":  "From-Process",
		"From-Module":   "From-Module",
		"Type":          "Type",
		"From":          "From",
		"Owner":         "Owner",
		"Anchor":        "Anchor",
		"Target":        "Target",
		"Data":          "Data",
		"Tags":          "Tags",
		"Read-Only":     "Read-Only",
	}
)

func isNonExtractableTag(tag string) bool {
	_, ok := NonExtractableTags[tag]
	return ok
}

func GetTagsValue(key string, tags []goarSchema.Tag) (t string) {
	for _, v := range tags {
		if v.Name == key {
			t = v.Value
			return
		}
	}
	return
}

func GetTagsValueByDefault(key string, tags []goarSchema.Tag, defaultValue string) (t string) {
	t = GetTagsValue(key, tags)
	if t == "" {
		t = defaultValue
	}
	return
}

func BaseToTags(b schema.Base) []goarSchema.Tag {
	return []goarSchema.Tag{
		{Name: "Data-Protocol", Value: b.DataProtocol},
		{Name: "Variant", Value: b.Variant},
		{Name: "Type", Value: b.Type},
	}
}

func TagsToBase(tags []goarSchema.Tag) (b schema.Base) {
	for _, v := range tags {
		switch v.Name {
		case "Data-Protocol":
			b.DataProtocol = v.Value
		case "Variant":
			b.Variant = v.Value
		case "Type":
			b.Type = v.Value
		}
	}
	return
}

func ModuleToTags(m schema.Module) (tags []goarSchema.Tag, err error) {
	tags = append(tags, BaseToTags(m.Base)...)
	tags = append(tags, []goarSchema.Tag{
		{Name: "Module-Format", Value: m.ModuleFormat},
		{Name: "Memory-Limit", Value: m.MemoryLimit},
		{Name: "Compute-Limit", Value: m.ComputeLimit},
	}...)
	if m.InputEncoding != "" {
		tags = append(tags, goarSchema.Tag{Name: "Input-Encoding", Value: m.InputEncoding})
	}
	if m.OutputEncoding != "" {
		tags = append(tags, goarSchema.Tag{Name: "Output-Encoding", Value: m.OutputEncoding})
	}
	tags = append(tags, m.Tags...)
	err = CheckDuplicateTag(tags)
	return
}

func TagsToModule(t []goarSchema.Tag) (m schema.Module, err error) {
	err = CheckDuplicateTag(t)
	if err != nil {
		return
	}

	b := TagsToBase(t)
	m.Base = b
	for _, v := range t {
		switch v.Name {
		case "Module-Format":
			m.ModuleFormat = v.Value
		case "Memory-Limit":
			m.MemoryLimit = v.Value
		case "Compute-Limit":
			m.ComputeLimit = v.Value
		case "Data-Protocol", "Variant", "Type":
			// skip tags in base
			continue
		default:
			// add other tags to Tags
			if v.Value != "" {
				m.Tags = append(m.Tags, v)
			}
		}
	}
	return
}

func ProcessToTags(p schema.Process) (tags []goarSchema.Tag, err error) {
	tags = append(tags, BaseToTags(p.Base)...)
	tags = append(tags, []goarSchema.Tag{
		{Name: "Module", Value: p.Module},
		{Name: "Scheduler", Value: p.Scheduler},
	}...)
	tags = append(tags, p.Tags...)
	err = CheckDuplicateTag(tags)
	return
}

func TagsToProcess(tags []goarSchema.Tag) (p schema.Process, err error) {
	err = CheckDuplicateTag(tags)
	if err != nil {
		return
	}

	b := TagsToBase(tags)
	p.Base = b
	for _, v := range tags {
		switch v.Name {
		case "Module":
			p.Module = v.Value
		case "Scheduler":
			p.Scheduler = v.Value
		case "From-Process":
			p.FromProcess = v.Value
		case "Data-Protocol", "Variant", "Type":
			// skip tags in base
			continue
		default:
			// remove NonExtractableTags
			if isNonExtractableTag(v.Name) {
				continue
			}
			// add other tags to Tags
			if v.Value != "" {
				p.Tags = append(p.Tags, v)
			}
		}
	}
	return
}

func MessageToTags(msg schema.Message) (tags []goarSchema.Tag, err error) {
	tags = append(tags, BaseToTags(msg.Base)...)
	if msg.Action != "" {
		tags = append(tags, goarSchema.Tag{Name: "Action", Value: msg.Action})
	}
	if msg.FromProcess != "" {
		tags = append(tags, goarSchema.Tag{Name: "From-Process", Value: msg.FromProcess})
	}
	if msg.PushedFor != "" {
		tags = append(tags, goarSchema.Tag{Name: "PushedFor", Value: msg.PushedFor})
	}
	if msg.Sequence != "" {
		tags = append(tags, goarSchema.Tag{Name: "Sequence", Value: msg.Sequence})
	}
	tags = append(tags, msg.Tags...)

	err = CheckDuplicateTag(tags)
	return
}

func TagsToMessage(tags []goarSchema.Tag) (m schema.Message, err error) {
	err = CheckDuplicateTag(tags)
	if err != nil {
		return
	}

	b := TagsToBase(tags)
	m.Base = b
	for _, v := range tags {
		switch v.Name {
		case "Action":
			m.Action = v.Value
		case "From-Process":
			m.FromProcess = v.Value
		case "Pushed-For":
			m.PushedFor = v.Value
		case "Sequence":
			m.Sequence = v.Value
		case "Data-Protocol", "Variant", "Type":
			// skip tags in base
			continue
		default:
			if isNonExtractableTag(v.Name) {
				continue
			}
			// add other tags to Tags
			if v.Value != "" {
				m.Tags = append(m.Tags, v)
			}
		}
	}
	return
}

func AssignmentToTags(a schema.Assignment) (tags []goarSchema.Tag, err error) {
	tags = append(tags, BaseToTags(a.Base)...)
	tags = append(tags, []goarSchema.Tag{
		{Name: "Process", Value: a.Process},
		{Name: "Message", Value: a.Message},
		{Name: "Nonce", Value: a.Nonce},
		{Name: "Timestamp", Value: a.Timestamp},
	}...)
	err = CheckDuplicateTag(tags)
	return
}

func TagsToAssignment(tags []goarSchema.Tag) (a schema.Assignment, err error) {
	err = CheckDuplicateTag(tags)
	if err != nil {
		return
	}

	b := TagsToBase(tags)
	a.Base = b
	for _, v := range tags {
		switch v.Name {
		case "Process":
			a.Process = v.Value
		case "Message":
			a.Message = v.Value
		case "Nonce":
			a.Nonce = v.Value
		case "Timestamp":
			a.Timestamp = v.Value
		}
	}
	return
}

func CheckpointToTags(c schema.Checkpoint) (tags []goarSchema.Tag, err error) {
	tags = append(tags, BaseToTags(c.Base)...)
	tags = append(tags, []goarSchema.Tag{
		{Name: "Process", Value: c.Process},
		{Name: "Nonce", Value: c.Nonce},
	}...)
	err = CheckDuplicateTag(tags)
	return
}

func TagsToCheckpoint(tags []goarSchema.Tag) (c schema.Assignment, err error) {
	err = CheckDuplicateTag(tags)
	if err != nil {
		return
	}

	b := TagsToBase(tags)
	c.Base = b
	for _, v := range tags {
		switch v.Name {
		case "Process":
			c.Process = v.Value
		case "Nonce":
			c.Nonce = v.Value
		}
	}
	return
}

func TagsToParams(tags []goarSchema.Tag) (params map[string]string, err error) {
	if err = CheckDuplicateTag(tags); err != nil {
		return
	}

	params = map[string]string{}
	for _, t := range tags {
		params[t.Name] = t.Value
	}

	return
}

func CheckDuplicateTag(tags []goarSchema.Tag) error {
	tagMap := make(map[string]bool)
	for _, v := range tags {
		if _, ok := tagMap[v.Name]; ok {
			return errors.New("duplicate tag: " + v.Name)

		}
		tagMap[v.Name] = true
	}
	return nil
}

// mergeTags combines two slices of tags while avoiding duplicates.
// If a tag name exists in both slices, the value from tags1 is preserved.
// Parameters:
//   - tags1: The primary slice of tags that takes precedence
//   - tags2: The secondary slice of tags to merge
//
// Returns:
//   - A new slice containing merged tags with no duplicate tag names
func MergeTags(tags1, tags2 []goarSchema.Tag) []goarSchema.Tag {
	// Create a map to track existing tag names from tags1
	tags1Map := make(map[string]string)
	for _, v := range tags1 {
		tags1Map[v.Name] = v.Value
	}

	// Add tags from tags2 only if their names don't exist in tags1
	for _, v := range tags2 {
		if _, ok := tags1Map[v.Name]; !ok {
			tags1 = append(tags1, v)
		}
	}
	return tags1
}
