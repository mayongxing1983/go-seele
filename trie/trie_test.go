/**
*  @file
*  @copyright defined in go-seele/LICENSE
 */
package trie

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/magiconair/properties/assert"
	"github.com/seeleteam/go-seele/common"
	"github.com/seeleteam/go-seele/database"
	"github.com/seeleteam/go-seele/database/leveldb"
)

func newTestTrieDB() (database.Database, func()) {
	dir, err := ioutil.TempDir("", "trietest")
	if err != nil {
		panic(err)
	}
	db, err := leveldb.NewLevelDB(dir)
	if err != nil {
		os.RemoveAll(dir)
		panic(err)
	}
	return db, func() {
		db.Close()
		os.RemoveAll(dir)
	}
}

func Test_trie_Update(t *testing.T) {
	db, remove := newTestTrieDB()
	defer remove()
	trie, err := NewTrie(common.Hash{}, []byte("trietest"), db)
	if err != nil {
		panic(err)
	}
	trie.Put([]byte("12345678"), []byte("test"))
	trie.Put([]byte("12345678"), []byte("testnew"))
	trie.Put([]byte("12345557"), []byte("test1"))
	trie.Put([]byte("12375879"), []byte("test2"))
	trie.Put([]byte("02375879"), []byte("test3"))
	trie.Put([]byte("04375879"), []byte("test4"))
	trie.Put([]byte("24375879"), []byte("test5"))
	trie.Put([]byte("24375878"), []byte("test6"))
	trie.Put([]byte("24355879"), []byte("test7"))
	value, _ := trie.Get([]byte("12345678"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "testnew")
	value, _ = trie.Get([]byte("12345557"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test1")
	value, _ = trie.Get([]byte("12375879"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test2")
	value, _ = trie.Get([]byte("02375879"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test3")
	value, _ = trie.Get([]byte("04375879"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test4")
	value, _ = trie.Get([]byte("24375879"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test5")
	value, _ = trie.Get([]byte("24375878"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test6")
	value, _ = trie.Get([]byte("24355879"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test7")
	batch := db.NewBatch()
	trie.Commit(batch)
	assert.Equal(t, err, nil)
	err = batch.Commit()
	assert.Equal(t, err, nil)

}

func Test_trie_Delete(t *testing.T) {
	db, remove := newTestTrieDB()
	defer remove()
	trie, err := NewTrie(common.Hash{}, []byte("trietest"), db)
	if err != nil {
		panic(err)
	}
	trie.Put([]byte("12345678123"), []byte("test"))
	trie.Put([]byte("12345557"), []byte("test1"))
	trie.Put([]byte("12375879321"), []byte("test2"))
	trie.Put([]byte("02375879"), []byte("test3"))
	trie.Put([]byte("04375879"), []byte("test4"))
	trie.Put([]byte("24375879"), []byte("test5"))
	trie.Put([]byte("24375878"), []byte("test6"))
	trie.Put([]byte("24355879"), []byte("test7"))
	match := trie.Delete([]byte("12345678123"))
	fmt.Println(match)
	assert.Equal(t, match, true)
	match = trie.Delete([]byte("12375879321"))
	fmt.Println(match)
	assert.Equal(t, match, true)
	match = trie.Delete([]byte("24375879"))
	fmt.Println(match)
	assert.Equal(t, match, true)
	match = trie.Delete([]byte("24375889"))
	fmt.Println(match)
	assert.Equal(t, match, false)
	value, _ := trie.Get([]byte("12345678123"))
	fmt.Println(string(value))
	assert.Equal(t, len(value), 0)
	value, _ = trie.Get([]byte("12375879321"))
	fmt.Println(string(value))
	assert.Equal(t, len(value), 0)
	value, _ = trie.Get([]byte("02375879"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test3")
	value, _ = trie.Get([]byte("04375879"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test4")
	value, _ = trie.Get([]byte("24375879"))
	fmt.Println(string(value))
	assert.Equal(t, len(value), 0)
	value, _ = trie.Get([]byte("24375878"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test6")
	value, _ = trie.Get([]byte("24355879"))
	assert.Equal(t, string(value), "test7")
	fmt.Println(string(value))
	fmt.Println(trie.Hash())
}

func Test_trie_Commit(t *testing.T) {
	db, remove := newTestTrieDB()
	defer remove()
	trie, err := NewTrie(common.Hash{}, []byte("trietest"), db)
	if err != nil {
		panic(err)
	}
	trie.Put([]byte("12345678"), []byte("test"))
	trie.Put([]byte("12345557"), []byte("test1"))
	trie.Put([]byte("12375879"), []byte("test2"))
	trie.Put([]byte("02375879"), []byte("test3"))
	trie.Put([]byte("04375879"), []byte("test4"))
	trie.Put([]byte("24375879"), []byte("test5"))
	trie.Put([]byte("24375878"), []byte("test6"))
	trie.Put([]byte("24355879"), []byte("test7"))

	batch := db.NewBatch()
	hash := trie.Commit(batch)
	batch.Commit()
	fmt.Println(hash)

	fmt.Println(string("----------------------------------"))
	trienew, err := NewTrie(hash, []byte("trietest"), db)

	trienew.Delete([]byte("24355879"))
	trienew.Put([]byte("243558790"), []byte("test8"))
	trienew.Put([]byte("043758790"), []byte("test9"))

	value, _ := trienew.Get([]byte("12345678"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test")
	value, _ = trienew.Get([]byte("12345557"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test1")
	value, _ = trienew.Get([]byte("12375879"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test2")
	value, _ = trienew.Get([]byte("02375879"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test3")
	value, _ = trienew.Get([]byte("04375879"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test4")
	value, _ = trienew.Get([]byte("24375879"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test5")
	value, _ = trienew.Get([]byte("24375878"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test6")
	value, _ = trienew.Get([]byte("24355879"))
	fmt.Println(string(value))
	assert.Equal(t, len(value), 0)
	value, _ = trienew.Get([]byte("243558790"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test8")
	value, _ = trienew.Get([]byte("043758790"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test9")
	value, _ = trienew.Get([]byte("12345557"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test1")
	value, _ = trienew.Get([]byte("12375879"))
	fmt.Println(string(value))
	assert.Equal(t, string(value), "test2")
}
