package main

func main() {

	keylock, err := LoadKeylock()
	if err != nil {
		panic(err)
	}

	taxonomy := NewTaxonomy(keylock)

	source, err := LoadSource(keylock, taxonomy)
	if err != nil {
		panic(err)
	}

	err = Build(source, taxonomy)
	if err != nil {
		panic(err)
	}

	err = keylock.Save()
	if err != nil {
		panic(err)
	}

}
