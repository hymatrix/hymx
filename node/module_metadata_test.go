package node

import (
	"strings"
	"testing"
)

func TestReadModuleTags(t *testing.T) {
	for _, prefix := range []string{
		`{`,
		`{"signatureType":3,"owner":"test",`,
		`{"data":"escaped\\\"value\u0041",`,
		`{"extra":{"tags":[{"name":"wrong"}],"values":[true,false,null,-1.25e2,{}]},`,
	} {
		t.Run(prefix, func(t *testing.T) {
			tags, err := readModuleTags(strings.NewReader(prefix + `"t\u0061gs":` + testModuleTags + `,"data":"unused"}`))
			if err != nil {
				t.Fatal(err)
			}
			if len(tags) != 5 || tags[3].Value != "test.runtime" {
				t.Fatalf("unexpected tags: %+v", tags)
			}
		})
	}
}

func TestReadModuleTagsRejectsInvalidHeaders(t *testing.T) {
	for _, input := range []string{
		`[]`, `{}`, `{"data":"unterminated`, `{"data":"bad\q","tags":[]}`,
		`{"data":"bad\u0X00","tags":[]}`, `{"data":wat,"tags":[]}`,
		`{"extra":[1,],"tags":[]}`, `{"extra":{"a" 1},"tags":[]}`,
		`{"tags":"wrong-type"}`, `{"extra":` + strings.Repeat("[", 130) + `0`,
		`{"tags":[{"name":"` + strings.Repeat("x", 1<<20) + `"}]}`,
	} {
		t.Run(input[:min(len(input), 60)], func(t *testing.T) {
			if _, err := readModuleTags(strings.NewReader(input)); err == nil {
				t.Fatal("invalid/oversized metadata accepted")
			}
		})
	}
}
