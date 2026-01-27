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

func GetTaxonomy(keylock *Keylock) *Taxonomy {
	return &Taxonomy{Keylock: keylock, Tags: []Tag{}}
}

func (taxonomy *Taxonomy) AssureLabelFromDocument(label string, document int) int {

	for i, tag := range taxonomy.Tags {
		if tag.Label == label {
			for _, mention := range tag.Mentions {
				if mention == document {
					return tag.Key
				}
			}

			taxonomy.Tags[i].Mentions = append(tag.Mentions, document)
			return tag.Key
		}
	}

	key := taxonomy.Keylock.AssureKey("TAG:" + label)

	taxonomy.Tags = append(taxonomy.Tags, Tag{
		Label:    label,
		Key:      key,
		Mentions: []int{document},
	})

	return key

}
