package mykv

type Tx struct {
	dirtyNodes    map[pgnum]*Node
	pagesToDelete []pgnum

	write bool

	db *DB

	root               pgnum
	freelist           *freelist
	collections        map[string]*Collection
	deletedCollections map[string]struct{}
}

func newTx(db *DB, write bool) *Tx {
	tx := &Tx{
		dirtyNodes:         map[pgnum]*Node{},
		pagesToDelete:      make([]pgnum, 0),
		write:              write,
		db:                 db,
		root:               db.root,
		collections:        map[string]*Collection{},
		deletedCollections: map[string]struct{}{},
	}
	if write {
		tx.freelist = db.freelist.clone()
	}
	return tx
}

func (tx *Tx) newNode(items []*Item, childNodes []pgnum) *Node {
	node := NewEmptyNode()
	node.items = items
	node.childNodes = childNodes
	node.pageNum = tx.freelist.getNextPage()
	node.tx = tx

	return node
}

func (tx *Tx) getNode(pageNum pgnum) (*Node, error) {
	if node, ok := tx.dirtyNodes[pageNum]; ok {
		return node, nil
	}

	node, err := tx.db.getNode(pageNum)
	if err != nil {
		return nil, err
	}
	node.tx = tx
	return node, nil
}

func (tx *Tx) writeNode(node *Node) *Node {
	tx.dirtyNodes[node.pageNum] = node
	node.tx = tx
	return node
}

func (tx *Tx) deleteNode(node *Node) {
	tx.pagesToDelete = append(tx.pagesToDelete, node.pageNum)
}

func (tx *Tx) Rollback() {
	if !tx.write {
		tx.db.commitLock.RUnlock()
		return
	}

	tx.dirtyNodes = nil
	tx.pagesToDelete = nil
	tx.freelist = nil
	tx.collections = nil
	tx.deletedCollections = nil
	tx.db.writeLock.Unlock()
}

func (tx *Tx) Commit() error {
	if !tx.write {
		tx.db.commitLock.RUnlock()
		return nil
	}
	defer tx.db.writeLock.Unlock()

	if err := tx.flushCollectionMetadata(); err != nil {
		return err
	}

	tx.db.commitLock.Lock()
	defer tx.db.commitLock.Unlock()

	pages, nextFreelist, nextMeta, err := tx.commitPages()
	if err != nil {
		return err
	}
	if err := tx.db.applyPagesWithWAL(pages); err != nil {
		return err
	}

	tx.db.freelist = nextFreelist
	tx.db.meta = nextMeta
	tx.dirtyNodes = nil
	tx.pagesToDelete = nil
	tx.freelist = nil
	tx.collections = nil
	tx.deletedCollections = nil
	return nil
}

func (tx *Tx) getRootCollection() *Collection {
	rootCollection := newEmptyCollection()
	rootCollection.root = tx.root
	rootCollection.tx = tx
	rootCollection.rootCollection = true
	return rootCollection
}

func (tx *Tx) GetCollection(name []byte) (*Collection, error) {
	key := string(name)
	if tx.write {
		if _, deleted := tx.deletedCollections[key]; deleted {
			return nil, nil
		}
		if collection, ok := tx.collections[key]; ok {
			return collection, nil
		}
	}

	rootCollection := tx.getRootCollection()
	item, err := rootCollection.Find(name)
	if err != nil {
		return nil, err
	}

	if item == nil {
		return nil, nil
	}

	collection := newEmptyCollection()
	collection.deserialize(item)
	collection.tx = tx
	tx.trackCollection(collection)
	return collection, nil
}

func (tx *Tx) CreateCollection(name []byte) (*Collection, error) {
	if !tx.write {
		return nil, errWriteInsideReadTx
	}

	newCollectionPage := tx.writeNode(tx.newNode(nil, nil))

	newCollection := newEmptyCollection()
	newCollection.name = name
	newCollection.root = newCollectionPage.pageNum
	return tx.createCollection(newCollection)
}

func (tx *Tx) GetOrCreateCollection(name []byte) (*Collection, error) {
	collection, err := tx.GetCollection(name)
	if err != nil {
		return nil, err
	}
	if collection != nil {
		return collection, nil
	}
	return tx.CreateCollection(name)
}

func (tx *Tx) DeleteCollection(name []byte) error {
	if !tx.write {
		return errWriteInsideReadTx
	}

	rootCollection := tx.getRootCollection()
	if err := rootCollection.Remove(name); err != nil {
		return err
	}
	delete(tx.collections, string(name))
	tx.deletedCollections[string(name)] = struct{}{}
	return nil
}

func (tx *Tx) createCollection(collection *Collection) (*Collection, error) {
	collection.tx = tx
	collectionBytes := collection.serialize()

	rootCollection := tx.getRootCollection()
	err := rootCollection.Put(collection.name, collectionBytes.value)
	if err != nil {
		return nil, err
	}
	delete(tx.deletedCollections, string(collection.name))
	tx.trackCollection(collection)

	return collection, nil
}

func (tx *Tx) trackCollection(collection *Collection) {
	if !tx.write || collection == nil || collection.rootCollection {
		return
	}
	tx.collections[string(collection.name)] = collection
}

func (tx *Tx) flushCollectionMetadata() error {
	for key, collection := range tx.collections {
		if _, deleted := tx.deletedCollections[key]; deleted || !collection.dirtyMeta {
			continue
		}
		rootCollection := tx.getRootCollection()
		item := collection.serialize()
		if err := rootCollection.Put(collection.name, item.value); err != nil {
			return err
		}
		collection.dirtyMeta = false
	}
	return nil
}

func (tx *Tx) commitPages() ([]*page, *freelist, *meta, error) {
	nextFreelist := tx.freelist.clone()
	for _, pageNum := range tx.pagesToDelete {
		nextFreelist.releasePage(pageNum)
	}

	pages := make([]*page, 0, len(tx.dirtyNodes)+2)
	for _, node := range tx.dirtyNodes {
		p, err := tx.db.nodePage(node)
		if err != nil {
			return nil, nil, nil, err
		}
		pages = append(pages, p)
	}

	freelistPage := tx.db.allocateEmptyPage()
	freelistPage.num = tx.db.freelistPage
	nextFreelist.serialize(freelistPage.data)
	pages = append(pages, freelistPage)

	nextMeta := &meta{
		root:         tx.root,
		freelistPage: tx.db.freelistPage,
	}
	metaPage := tx.db.allocateEmptyPage()
	metaPage.num = metaPageNum
	nextMeta.serialize(metaPage.data)
	pages = append(pages, metaPage)

	return pages, nextFreelist, nextMeta, nil
}
