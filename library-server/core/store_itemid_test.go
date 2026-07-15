package main

import "testing"

func TestStoreReaderStateByItemID(t *testing.T) {
	st, err := openStore(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer st.db.Close()

	// El estado personal se scopea por usuario (PK con user); "" = invitado.
	// Este test verifica el direccionamiento por itemId dentro de un namespace.
	const user = ""
	const itemID = "media:published/books/linux.pdf"
	if got := canonicalItemID("", itemLegacyLib, itemID); got != itemID {
		t.Fatalf("canonicalItemID internal item = %q, want %q", got, itemID)
	}
	if got := canonicalItemID("", "wikipedia_es", ""); got != "" {
		t.Fatalf("canonicalItemID partial legacy key = %q, want empty", got)
	}

	if err := st.PutFavorite(user, Fav{ItemID: itemID, Title: "Linux Bible", Book: "Books", OnHome: true}, 100); err != nil {
		t.Fatalf("PutFavorite: %v", err)
	}
	favs, err := st.ListFavorites(user)
	if err != nil {
		t.Fatalf("ListFavorites: %v", err)
	}
	// El centinela interno __item__ no debe salir por la API: el contenido nativo
	// de Item se expone solo por itemId, con lib/path vacíos.
	if len(favs) != 1 || favs[0].ItemID != itemID || favs[0].Lib != "" || favs[0].Path != "" {
		t.Fatalf("favorites = %+v", favs)
	}
	if err := st.DeleteFavorite(user, "", "", itemID); err != nil {
		t.Fatalf("DeleteFavorite: %v", err)
	}
	favs, _ = st.ListFavorites(user)
	if len(favs) != 0 {
		t.Fatalf("favorites after delete = %+v", favs)
	}

	if err := st.PutNote(user, Note{ItemID: itemID, Title: "Linux Bible", Book: "Books", Body: "note"}, 200); err != nil {
		t.Fatalf("PutNote: %v", err)
	}
	note, err := st.GetNote(user, "", "", itemID)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if note == nil || note.ItemID != itemID || note.Body != "note" {
		t.Fatalf("note = %+v", note)
	}
	if err := st.DeleteNote(user, "", "", itemID); err != nil {
		t.Fatalf("DeleteNote: %v", err)
	}
	note, _ = st.GetNote(user, "", "", itemID)
	if note != nil {
		t.Fatalf("note after delete = %+v", note)
	}

	if err := st.AddHistory(user, Visit{ItemID: itemID, Title: "Linux Bible", Book: "Books"}, 300); err != nil {
		t.Fatalf("AddHistory: %v", err)
	}
	recent, err := st.ListRecent(user, 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(recent) != 1 || recent[0].ItemID != itemID {
		t.Fatalf("recent = %+v", recent)
	}
	if err := st.DeleteHistoryPath(user, "", "", itemID); err != nil {
		t.Fatalf("DeleteHistoryPath: %v", err)
	}
	recent, _ = st.ListRecent(user, 10)
	if len(recent) != 0 {
		t.Fatalf("recent after delete = %+v", recent)
	}

	if err := st.AddTag(user, Tag{ItemID: itemID, Tag: "linux", Title: "Linux Bible", Book: "Books"}, 400); err != nil {
		t.Fatalf("AddTag: %v", err)
	}
	pageTags, err := st.PageTags(user, "", "", itemID)
	if err != nil {
		t.Fatalf("PageTags: %v", err)
	}
	if len(pageTags) != 1 || pageTags[0] != "linux" {
		t.Fatalf("pageTags = %+v", pageTags)
	}
	keys, err := st.TaggedKeys(user)
	if err != nil {
		t.Fatalf("TaggedKeys: %v", err)
	}
	if len(keys) != 1 || keys[0] != itemID {
		t.Fatalf("keys = %+v", keys)
	}
	if err := st.RemoveTag(user, "", "", itemID, "linux"); err != nil {
		t.Fatalf("RemoveTag: %v", err)
	}
	pageTags, _ = st.PageTags(user, "", "", itemID)
	if len(pageTags) != 0 {
		t.Fatalf("pageTags after delete = %+v", pageTags)
	}
}
