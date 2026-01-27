package main

func main() {

	keylock, err := GetKeylock()
	if err != nil {
		panic(err)
	}

	taxonomy := GetTaxonomy(keylock)

	source, err := GetSource(keylock, taxonomy)
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
