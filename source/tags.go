package main

type Tag struct {
	Label    string
	Key      int
	Mentions []int
}

type Taxonomy struct {
	Keylock *Keylock
	Tags    []Tag
}

func NewTaxonomy(keylock *Keylock) *Taxonomy {
	return &Taxonomy{Keylock: keylock, Tags: []Tag{}}
}

func (taxonomy *Taxonomy) AssureTag(label string) *Tag {
	for i := range taxonomy.Tags {
		if taxonomy.Tags[i].Label == label {
			return &taxonomy.Tags[i]
		}
	}
	key := taxonomy.Keylock.AssureKey("TAG:" + label)
	taxonomy.Tags = append(taxonomy.Tags, Tag{
		Label:    label,
		Key:      key,
		Mentions: []int{},
	})
	return &taxonomy.Tags[len(taxonomy.Tags)-1]
}

func (tag *Tag) AssureMention(document int) {
	for _, mention := range tag.Mentions {
		if mention == document {
			return
		}
	}
	tag.Mentions = append(tag.Mentions, document)
}
