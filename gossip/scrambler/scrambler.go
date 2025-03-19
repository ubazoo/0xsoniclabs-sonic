package scrambler

import (
	"cmp"
	"iter"
	"maps"
	"math/rand/v2"
	"slices"
)

// GetExecutionOrder returns the order in which transactions should be processed.
// If processed in this order, the transactions can be processed without violating
// any dependencies. Transactions which cannot be processed due to missing
// dependencies are not included in the result.
//
// Beyond the order defined by dependencies, the order of transactions is
// randomized in a deterministic way. This is to avoid the risk of transaction
// ordering attacks.
func GetExecutionOrder(
	transactions []transaction,
	sort func([]transaction, ...tieBreaker) []transaction,
	pick ...tieBreaker,
) []transaction {

	// Step 1: remove duplicates and replacements
	transactions = deduplicate(transactions)
	transactions = removeMuteAuthorizations(transactions)

	// Step 2: identify partitioning of transactions such that each partition is
	// a set of transactions that have dependencies only within the partition.
	partitions := partition(transactions)

	// Step 3: sort transactions within partitions respecting the dependencies.
	// This step may also remove transactions that can not be processed.
	for i, partition := range partitions {
		partitions[i] = sort(partition, pick...)
	}

	// Step 4: interleave partitions using a random order.
	// --- !! ATTENTION !! ---
	// TODO: to make this function deterministic the partitions need to be
	// sorted before interleaving them. This is not implemented yet.
	// --- !! ATTENTION !! ---
	return interleavePartitions(partitions)
}

type transaction struct {
	main action
	auth []action
}

func (tx transaction) actions() []action {
	return append([]action{tx.main}, tx.auth...)
}

type action struct {
	sender int
	nonce  int
}

func (a action) predecessor() action {
	return action{a.sender, a.nonce - 1}
}

func (a action) successor() action {
	return action{a.sender, a.nonce + 1}
}

// --- Deduplication ---

// deduplicate removes duplicates and replacements from the list of transactions.
func deduplicate(transactions []transaction) []transaction {

	// WARNING: this function is removing duplicates and replacements based on
	// the main action only. It does not consider transaction hashes or gas
	// prices as a real-world implementation would have to do.

	seen := map[action]unit{}
	res := make([]transaction, 0, len(transactions))
	for _, tx := range transactions {
		if _, found := seen[tx.main]; found {
			continue
		}
		seen[tx.main] = unit{}
		res = append(res, tx)
	}

	return res
}

// removeMuteAuthorizations removes authorizations that can never be successful.
// Examples are authorizations that have the same sender and nonce as the
// transaction itself or are duplicates of other authorizations.
//
// This function is O(|actions|).
func removeMuteAuthorizations(transactions []transaction) []transaction {
	seen := map[action]unit{}
	for i := range transactions {
		tx := transactions[i]
		res := transaction{
			main: tx.main,
		}
		for _, auth := range tx.auth {
			// authorizations to the transactions sender account must have
			// a higher nonce than the transaction itself.
			if auth.sender == tx.main.sender && auth.nonce <= tx.main.sender {
				continue
			}
			// duplicates can be ignored for the scheduling
			if _, found := seen[auth]; found {
				continue
			}
			seen[auth] = unit{}
			res.auth = append(res.auth, auth)
		}
		transactions[i] = res
	}
	return transactions
}

// --- Partitioning ---

// partition partitions the transactions into groups of transactions that can be
// sorted independently. Transactions without any inter-dependency in their
// execution order end up in different lists. Transactions with
// inter-dependencies end up in the same sub-list.
func partition(transactions []transaction) [][]transaction {
	return partition2(transactions)
}

func partition1(transactions []transaction) [][]transaction {

	// WARNING: This is a proof-of-concept that is not necessarily efficient.
	// It is a simple implementation that is not optimized to minimize the
	// worst-case runtime complexity. It is intended for prototype purposes
	// only.

	// Initially, every transaction is its own partition.
	partition := make([]int, len(transactions))
	for i := range transactions {
		partition[i] = i
	}

	// We merge partitions until no more merges are possible.
	for {
		merged := false

		for i, txA := range transactions {
			for j, txB := range transactions {
				if i == j {
					continue
				}

				// if the transaction are already in the same partition, we can
				// skip another check.
				if partition[i] == partition[j] {
					continue
				}

				// test whether there is a dependency between the two transactions.
				if !dependsOn(txA, txB) {
					continue
				}

				// merge the partitions of the two transactions.
				new := partition[i]
				old := partition[j]
				if old < new {
					new, old = old, new
				}
				for k := range partition {
					if partition[k] == old {
						partition[k] = new
					}
				}

				merged = true
			}
		}

		if !merged {
			break
		}
	}

	grouped := map[int][]transaction{}
	for i, p := range partition {
		grouped[p] = append(grouped[p], transactions[i])
	}

	res := make([][]transaction, 0, len(grouped))
	for _, group := range grouped {
		res = append(res, group)
	}
	return res
}

func dependsOn(a, b transaction) bool {
	for _, a := range a.actions() {
		for _, b := range b.actions() {
			if a.sender == b.sender {
				return true
			}
		}
	}
	return false
}

type unit struct{}

func partition2(transactions []transaction) [][]transaction {

	// Partitions the given list of transactions into groups of
	// interdependent transactions. Transactions within different groups can be
	// processed in an arbitrary order, while transactions within the same group
	// have interdependencies that need to be sorted.

	type sender = int
	type tx = int
	type component = int

	// Step 1: create a sender -> transaction index mapping.
	// This step is O(|actions|).
	senderToTx := map[sender][]tx{}
	for i, tx := range transactions {
		for _, a := range tx.actions() {
			senderToTx[a.sender] = append(senderToTx[a.sender], i)
		}
	}

	// Step 2: create a graph G = (N, E) where
	// - N is the set of sender addresses
	// - E = { {a, b} | a, b in N, a != b, there is a transaction touching a and b }
	// For this we have
	// - |N| <= |actions|
	// - |E| <= |actions|
	graph := map[sender]map[sender]unit{}
	// This loop is O(|senderToTx|) = O(|actions|).
	for s := range senderToTx {
		graph[s] = map[sender]unit{}
	}
	// This loop is O(|actions|).
	for _, tx := range transactions {
		// By connecting the senders of the actions in the transaction, in a
		// chain-like fashion, we indicate that all of these senders are
		// required to be part of the same connected component. A simple chain
		// is sufficient for computing components, we do not need a fully
		// connected graph.
		actions := tx.actions()
		for i := range len(actions) - 1 {
			a := actions[i]
			b := actions[i+1]
			if a.sender == b.sender {
				continue
			}
			graph[a.sender][b.sender] = unit{}
			graph[b.sender][a.sender] = unit{}
		}
	}

	// Step 3: find connected components in the graph.
	// This step is O(|E|) = O(|actions|).
	components := map[sender]component{}
	numComponents := 0
	for sender := range graph {
		if _, found := components[sender]; found {
			continue
		}
		component := numComponents
		numComponents++
		for element := range getConnectedNodes(graph, sender) {
			components[element] = component
		}
	}

	// Step 4: group transactions by connected components.
	// This step is O(|transactions|).
	res := make([][]transaction, numComponents)
	for _, tx := range transactions {
		component := components[tx.main.sender]
		res[component] = append(res[component], tx)
	}

	return res
}

func getConnectedNodes[N comparable](graph map[N]map[N]unit, seed N) iter.Seq[N] {
	return func(yield func(N) bool) {
		seen := map[N]unit{}
		workList := []N{seed}
		for len(workList) > 0 {
			node := workList[0]
			workList = workList[1:]
			if !yield(node) {
				return
			}
			seen[node] = unit{}
			for neighbor := range graph[node] {
				if _, seen := seen[neighbor]; !seen {
					workList = append(workList, neighbor)
				}
			}
		}
	}
}

// --- Sorting ---

func sortPartition(partition []transaction, tieBreaker ...tieBreaker) []transaction {

	// WARNING: This is a proof-of-concept that is not necessarily efficient.
	// It is a simple implementation that is not optimized to minimize the
	// worst-case runtime complexity. It is intended for prototype purposes
	// only.

	// We start by determining the current nonces of all senders referenced in
	// the partition. Here, we assume that the initial nonce is the smallest
	// nonce that is referenced by any transaction in the partition. This could
	// be improved by fetching the actual nonce from the database.
	nonces := getPresumedInitialState(partition)

	// Select the heuristic to be used to break ties when multiple transactions
	// can be processed at the same time.
	pick := pickFirst
	if len(tieBreaker) > 0 {
		pick = tieBreaker[0]
	}

	// Use the selected heuristic to sort the transactions.
	return getTransactionOrder(partition, nonces, pick)
}

// getTransactionOrder returns the order in which transactions should be
// processed. If processed in this order, the transactions can be processed
// without violating any dependencies. Transactions which cannot be processed
// due to missing dependencies are not included in the result.
//
// The provided tie breaker is used to select to pick one out of multiple
// options in case multiple transactions can be processed at the same time.
// This function is deterministic if the tie breaker is deterministic.
func getTransactionOrder(
	transactions []transaction,
	nonces state,
	pick tieBreaker,
) []transaction {
	// From here on, we iteratively search for transactions that can be
	// processed and select among those the one that should be processed next.
	// We continue until no more transactions can be processed.
	var res []transaction
	for {
		// Find all transactions that can be processed.
		ready := []transaction{}
		for _, tx := range transactions {
			if nonces[tx.main.sender] == tx.main.nonce {
				ready = append(ready, tx)
			}
		}
		if len(ready) == 0 {
			return res
		}

		// Select the transaction that should be processed next.
		next := pick(ready, transactions, nonces)
		res = append(res, next)

		// Update the nonces of the senders.
		nonces.apply(next)
	}
}

// tieBreaker is a function that selects one out of multiple transactions that
// can be processed at the same time.
type tieBreaker func(options []transaction, all []transaction, nonces state) transaction

// state is a utility type to track the current nonce of each account while
// sorting transactions.
type state map[int]int // account -> nonce

// getPresetInitialState returns the initial state of nonces based on the
// transactions. This function assumes that the initial nonce is the smallest
// nonce that is referenced by any transaction in the list of transactions.
//
// This function is O(|actions|).
func getPresumedInitialState(transactions []transaction) state {
	// O(|actions|)
	transactions = removeMuteAuthorizations(transactions)
	// Compute the initial state of nonces indicated by the transactions.
	state := state{}
	for _, tx := range transactions { // O(|actions|)
		for _, a := range tx.actions() {
			if nonce, found := state[a.sender]; !found || a.nonce < nonce {
				state[a.sender] = a.nonce
			}
		}
	}
	return state
}

func (s *state) apply(tx transaction) (bool, int) {
	// Check that the transaction can be processed.
	if (*s)[tx.main.sender] != tx.main.nonce {
		return false, 0
	}
	(*s)[tx.main.sender]++

	// Apply the authorizations.
	passedAuthorizations := 0
	for _, a := range tx.auth {
		cur := (*s)[a.sender]
		if a.nonce == cur {
			(*s)[a.sender]++
			passedAuthorizations++
		}
	}
	return true, passedAuthorizations
}

func (s state) copy() state {
	return maps.Clone(s)
}

// pickFirst is a trivial tie breaker that always picks the first transaction.
func pickFirst(
	ready []transaction,
	_ []transaction,
	_ state,
) transaction {
	return ready[0]
}

type actionKind int

const (
	actionKind_Transaction actionKind = iota
	actionKind_Authorization
)

// pickHighestPotential is a tie breaker that selects the transaction that has
// the longest chain of transactions and authorizations depending on it.
func pickHighestPotential(
	ready []transaction,
	all []transaction,
	_ state,
) transaction {
	if len(ready) == 1 {
		return ready[0]
	}

	// Index all actions to simplify the search for dependencies.
	actions := map[action]actionKind{}
	for _, tx := range all {
		actions[tx.main] = actionKind_Transaction
		for _, a := range tx.auth {
			if _, found := actions[a]; !found {
				actions[a] = actionKind_Authorization
			}
		}
	}

	potential := func(a action) score {
		// Authorizations with the same nonce of a transaction are preventing
		// the transaction from being processed and are thus scored negatively.
		if kind := actions[a]; kind == actionKind_Transaction {
			return score{numTransactions: -1}
		}
		potential := score{}
		for i := a.nonce; ; i++ {
			kind, found := actions[action{a.sender, i}]
			if !found {
				break
			}
			if kind == actionKind_Transaction {
				potential.numTransactions++
			} else {
				potential.numAuthorizations++
			}
		}
		return potential
	}

	// Compute the potential of each transaction.
	type sender = int
	txPotential := make([]score, len(ready))
	for i, tx := range ready {
		perSender := map[sender]score{}
		for _, a := range tx.auth {
			if a == tx.main {
				continue
			}
			got := potential(a)
			if cur, found := perSender[a.sender]; !found {
				perSender[a.sender] = got
			} else {
				if got.isBetterThan(cur) {
					perSender[a.sender] = got
				}
			}
		}
		potential := score{}
		for _, p := range perSender {
			potential.numTransactions += p.numTransactions
			potential.numAuthorizations += p.numAuthorizations
		}
		txPotential[i] = potential
	}

	// pick the transaction with the highest potential
	bestPotential := txPotential[0]
	bestTransactionPosition := 0
	for i, p := range txPotential {
		if p.isBetterThan(bestPotential) {
			bestPotential = p
			bestTransactionPosition = i
		}
	}

	return ready[bestTransactionPosition]
}

// pickOptimal is a tie breaker that selects the transaction that allows for the
// most transactions to be processed in total. This implementation is producing
// the optimal result. However, its runtime may be exponential in the size of
// the total set of transactions.
func pickOptimal(
	ready []transaction,
	transactions []transaction,
	nonces state,
) transaction {
	var best transaction
	bestScore := score{}
	for _, tx := range ready {
		// Copy the state to avoid modifying the original state.
		state := nonces.copy()

		// apply the transaction
		_, numAuthorizations := state.apply(tx)

		order := getTransactionOrder(transactions, state.copy(), pickOptimal)
		score := eval(state.copy(), order)

		// add the score of the current transaction to the total score
		score.numTransactions++
		score.numAuthorizations += numAuthorizations

		if score.isBetterThan(bestScore) {
			best = tx
			bestScore = score
		}
	}
	return best
}

// score is a summary of the quality of a transaction order.
type score struct {
	numTransactions   int
	numAuthorizations int
}

func (s score) isBetterThan(other score) bool {
	return s.numTransactions > other.numTransactions ||
		(s.numTransactions == other.numTransactions &&
			s.numAuthorizations > other.numAuthorizations)
}

// eval computes the score of a given order of transactions for the given
// initial state of nonces.
func eval(nonces state, order []transaction) score {
	score := score{}
	state := maps.Clone(nonces)
	for _, tx := range order {
		if state[tx.main.sender] != tx.main.nonce {
			continue
		}
		state[tx.main.sender]++
		score.numTransactions++
		for _, a := range tx.auth {
			if state[a.sender] != a.nonce {
				continue
			}
			state[a.sender]++
			score.numAuthorizations++
		}
	}
	return score
}

// --- Sorting 2 ---

// This implementation is an improvement on the sorting algorithm above by
// retaining precomputed information about enabled actions between consecutive
// calls of the pickNext function. This allows for a more efficient selection of
// the next transaction to be processed.

func sortPartition2(partition []transaction, _ ...tieBreaker) []transaction {

	// WARNING: This is a proof-of-concept that is not necessarily efficient.
	// It is a simple implementation that is not optimized to minimize the
	// worst-case runtime complexity. It is intended for prototype purposes
	// only.

	// We start by determining the current nonces of all senders referenced in
	// the partition. Here, we assume that the initial nonce is the smallest
	// nonce that is referenced by any transaction in the partition. This could
	// be improved by fetching the actual nonce from the database.
	nonces := getPresumedInitialState(partition)

	// track the set of enabled actions
	enabled := map[action]unit{}
	for sender, nonce := range nonces {
		enabled[action{sender, nonce}] = unit{}
	}

	// create the execution order step by step
	res := []transaction{}
	for {
		// get list of all transactions that can be processed
		ready := []transaction{}
		for _, tx := range partition {
			if _, found := enabled[tx.main]; found {
				ready = append(ready, tx)
			}
		}
		if len(ready) == 0 {
			break
		}

		// select the transaction to be processed next
		next := pickNext(ready, enabled)

		res = append(res, next)

		// update set of enabled actions
		for _, a := range next.actions() {
			if _, found := enabled[a]; found {
				delete(enabled, a)
				enabled[action{a.sender, a.nonce + 1}] = unit{}
			}
		}
	}

	return res
}

func pickNext(ready []transaction, enabled map[action]unit) transaction {
	// compute all transactions currently pending
	pendingTransactions := map[action]unit{}
	for _, tx := range ready {
		pendingTransactions[tx.main] = unit{}
	}

	// look for candidates where all authorizations are enabled
	candidates := []transaction{}
	for _, tx := range ready {
		ready := true
		for _, a := range tx.auth {
			// if there is a transaction pending for this authorization, ignore
			// the current transaction unless it is the current transaction
			// itself.
			if tx.main != a {
				if _, found := pendingTransactions[a]; found {
					ready = false
					break
				}
			}
			if a != (action{tx.main.sender, tx.main.nonce + 1}) {
				if _, found := enabled[a]; !found {
					ready = false
					break
				}
			}
		}
		if ready {
			candidates = append(candidates, tx)
		}
	}

	// If there are no candidates with all authorizations enabled, favor
	// those transactions that do not have any authorizations disabling a
	// pending transaction.
	if len(candidates) == 0 {
		for _, tx := range ready {
			ready := true
			for _, a := range tx.auth {
				if tx.main != a {
					if _, found := pendingTransactions[a]; found {
						ready = false
						break
					}
				}
			}
			if ready {
				candidates = append(candidates, tx)
			}
		}
	}

	// If there are no fully enabled candidates, consider all ready transactions
	// as candidates.
	if len(candidates) == 0 {
		candidates = ready
	}

	// pick the one with the highest number of authorizations
	// TODO: refine this ...
	max := 0
	res := candidates[0]
	for _, tx := range candidates {
		if len(tx.auth) > max {
			max = len(tx.auth)
			res = tx
		}
	}
	return res
}

// --- Sorting 3 ---

func sortPartition3(partition []transaction, _ ...tieBreaker) []transaction {

	// WARNING: This is a proof-of-concept that is not necessarily efficient.
	// It is a simple implementation that is not optimized to minimize the
	// worst-case runtime complexity. It is intended for prototype purposes
	// only.

	// We start by determining the current nonces of all senders referenced in
	// the partition. Here, we assume that the initial nonce is the smallest
	// nonce that is referenced by any transaction in the partition. This could
	// be improved by fetching the actual nonce from the database.
	nonces := getPresumedInitialState(partition) // O(|actions|)

	// Remove all un-reachable transactions from the partition.
	partition = filterUnreachable(partition, nonces) // O(|actions|)

	// create a set of pending transactions
	pending := map[action]unit{}
	for _, tx := range partition { // O(|transactions|) <= O(|actions|)
		pending[tx.main] = unit{}
	}

	// Initialize the list of transactions sorted by their evaluation.
	queue := newHeap[action](func(a, b valuedTransaction) int {
		return b.value.compare(a.value) // minimum!
	})
	for _, tx := range partition { // O(|actions|)
		// tx.main is unique, since we removed duplicates and replacements
		queue.Add(tx.main, valuedTransaction{
			tx:    tx,
			value: evaluate(tx, nonces, pending),
		})
	}

	// Create a map of dependencies linking actions to transactions they may
	// be affecting.
	dependencies := map[action][]transaction{}
	for _, tx := range partition { // O(|actions|)
		dependencies[tx.main.predecessor()] = append(dependencies[tx.main.predecessor()], tx)
		dependencies[tx.main] = append(dependencies[tx.main], tx)
		dependencies[tx.main.successor()] = append(dependencies[tx.main.successor()], tx)
		for _, a := range tx.auth {
			dependencies[a.predecessor()] = append(dependencies[a.predecessor()], tx)
			dependencies[a] = append(dependencies[a], tx)
			dependencies[a.successor()] = append(dependencies[a.successor()], tx)
		}
	}

	// create the execution order step by step
	res := []transaction{}
	// This loop has a complexity of
	//   O(max(|actions| * log(|transactions|, |transactions| * log(|transactions|)))
	// which is O(|actions| * log(|actions|)).
	for { // up to |transactions| iterations

		// Pick the top candidate and check if it can be processed.
		next, found := queue.Pop() // O(log(|transactions|))
		if !found || !next.value.runnable {
			break
		}

		// Schedule the selected transaction and apply effects.
		res = append(res, next.tx)
		delete(pending, next.tx.main)

		// Update the evaluation of all candidates affected by the scheduled
		// transaction.
		// The following loops have a combined complexity of
		// O(max(|actions|*log(|transactions|), |actions|*O(|tx.auth|)))
		// over all iterations of the outer loop. This is because each
		// dependency is processed at most once and there are at most
		// 3*|actions| dependencies.
		nonces[next.tx.main.sender]++
		for _, tx := range dependencies[next.tx.main] {
			queue.Update(tx.main, valuedTransaction{ // O(log(|transactions|))
				tx:    tx,
				value: evaluate(tx, nonces, pending), // O(|tx.auth|)
			})
		}
		delete(dependencies, next.tx.main)
		for _, a := range next.tx.auth {
			if nonces[a.sender] == a.nonce {
				nonces[a.sender]++
				for _, tx := range dependencies[a] {
					queue.Update(tx.main, valuedTransaction{ // O(log(|transactions|))
						tx:    tx,
						value: evaluate(tx, nonces, pending), // O(|tx.auth|)
					})
				}
				delete(dependencies, a)
			}
		}
	}

	return res
}

// filterUnreachable removes transactions that can not be processed in any
// schedule due to missing dependencies. In particular, if a transaction has
// a sender s and nonce n, and the current nonce is not n-1 nor is there an
// action with sender s and nonce n-1, the transaction can not be processed.
//
// This function is O(|actions|).
func filterUnreachable(tx []transaction, nonces state) []transaction {
	releases := map[action]unit{}
	for _, tx := range tx { // O(|actions|)
		releases[tx.main] = unit{}
		for _, a := range tx.auth {
			releases[a] = unit{}
		}
	}

	res := []transaction{}
	for _, tx := range tx { // O(|transactions|) <= O(|actions|)
		if nonces[tx.main.sender] == tx.main.nonce {
			res = append(res, tx)
			continue
		}
		requirement := action{tx.main.sender, tx.main.nonce - 1}
		if _, found := releases[requirement]; found {
			res = append(res, tx)
		}
	}
	return res
}

// value defines the evaluation result used for defining the scheduling priority
// of transactions.
type value struct {
	runnable                                          bool
	numAuthorizationsCollidingWithPendingTransactions int
	numBlockedAuthorizations                          int
	numAuthorizations                                 int
}

func (v value) compare(other value) int {
	if v.runnable != other.runnable {
		if v.runnable {
			return -1
		} else {
			return 1
		}
	}
	if res := cmp.Compare(v.numAuthorizationsCollidingWithPendingTransactions, other.numAuthorizationsCollidingWithPendingTransactions); res != 0 {
		return res
	}
	if res := cmp.Compare(v.numBlockedAuthorizations, other.numBlockedAuthorizations); res != 0 {
		return res
	}
	return -cmp.Compare(v.numAuthorizations, other.numAuthorizations)
}

type valuedTransaction struct {
	tx    transaction
	value value
}

// evaluate computes the value of a transaction based on the current state of
// nonces and the set of pending transactions.
//
// This function is O(|tx.auth|).
func evaluate(
	tx transaction,
	nonces state,
	pending map[action]unit,
) value {

	value := value{
		runnable:          nonces[tx.main.sender] == tx.main.nonce,
		numAuthorizations: len(tx.auth),
	}

	for _, a := range tx.auth {
		if _, found := pending[a]; found {
			if a.nonce == nonces[a.sender] {
				value.numAuthorizationsCollidingWithPendingTransactions++
			}
		}
		if a.nonce > nonces[a.sender] {
			value.numBlockedAuthorizations++
		}
	}

	return value
}

// --- Interleaving ---

func interleavePartitions[T any](partition [][]T) []T {
	return interleavePartitions3(partition)
}

func interleavePartitions1[T any](partition [][]T) []T {

	// WARNING: This is just a dummy implementation; the real implementation
	// should use a random source to sample from partitions in a random order.

	length := 0
	for _, partition := range partition {
		length += len(partition)
	}
	if length == 0 {
		return nil
	}

	res := make([]T, 0, length)
	for i := range length {
		i := i // TODO: add random source here
		for j := range len(partition) {
			pos := (i + j) % len(partition)
			if len(partition[pos]) > 0 {
				res = append(res, partition[pos][0])
				partition[pos] = partition[pos][1:]
				break
			}
		}
	}
	return res
}

func interleavePartitions2[T any](partition [][]T) []T {
	return interleavePartitions2WithSeed(0, partition)
}

func interleavePartitions2WithSeed[T any](seed uint64, partition [][]T) []T {

	// Step 1: Create a list of proxies that represent the partitions.
	numTransactions := 0
	// This loop is O(|partition|) = O(|transactions|).
	for _, part := range partition {
		numTransactions += len(part)
	}
	if numTransactions == 0 {
		return nil
	}

	proxies := make([]int, 0, numTransactions)
	// This loop nest is O(|transactions|).
	for i, part := range partition {
		for range len(part) {
			proxies = append(proxies, i)
		}
	}

	rand := xorShiftStar{}
	rand.seed(seed)

	// Step2: Shuffle the proxies.
	// This is an implementation of the Fisher-Yates shuffle algorithm.
	// See: https://en.wikipedia.org/wiki/Fisher%E2%80%93Yates_shuffle
	// This loop is O(|transactions|).
	for i := len(proxies) - 1; i > 0; i-- {
		j := rand.randN(uint64(i + 1)) // TODO: add random source here, picking a number in [0, i) uniformly
		proxies[i], proxies[j] = proxies[j], proxies[i]
	}

	// Step 3: Create the result by interleaving the parts of the partition
	// according to the shuffled proxies.
	res := make([]T, 0, numTransactions)
	// This loop is O(|transactions|).
	for _, proxy := range proxies {
		res = append(res, partition[proxy][0])
		partition[proxy] = partition[proxy][1:]
	}
	return res
}

func interleavePartitions3[T any](partition [][]T) []T {
	seed := uint64(0xaaAAaaAAaa) // TODO: add random seed here
	return interleavePartitions3WithSeed(seed, partition)
}

func interleavePartitions3WithSeed[T any](seed uint64, partition [][]T) []T {

	// This implementation determines the random order of interleaving by
	// through the following algorithm:
	//  - given, a random seed R
	//  - we create the a list [0,...,N-1] where N is the number of transactions
	//  - we xor each element of the list with R
	//  - we sort the list
	//  - we use the sorted list to determine the order of transactions
	//
	// See https://en.wikipedia.org/wiki/Fisher%E2%80%93Yates_shuffle#Sorting
	// for a discussion of this algorithm.

	// Step 1: Create a list of proxies that represent the partitions.
	numTransactions := 0
	// This loop is O(|partition|) = O(|transactions|).
	for _, part := range partition {
		numTransactions += len(part)
	}
	if numTransactions == 0 {
		return nil
	}

	type proxy struct {
		pos  uint
		part int
	}
	proxies := make([]proxy, 0, numTransactions)
	// This loop nest is O(|transactions|).
	for i, part := range partition {
		for range len(part) {
			pos := uint(len(proxies)) ^ uint(seed)
			proxies = append(proxies, proxy{pos, i})
		}
	}

	// Step2: Shuffle the proxies by sorting them according to the seeded position.
	// This step is O(NlogN) where N is |transactions|.
	// If we can find a Radix sort implementation, this could be reduced to O(N).
	slices.SortFunc(proxies, func(a, b proxy) int {
		return cmp.Compare(a.pos, b.pos)
	})

	// Step 3: Create the result by interleaving the parts of the partition
	// according to the shuffled proxies.
	res := make([]T, 0, numTransactions)
	// This loop is O(|transactions|).
	for _, proxy := range proxies {
		res = append(res, partition[proxy.part][0])
		partition[proxy.part] = partition[proxy.part][1:]
	}
	return res
}

func interleavePartitions4[T any](partition [][]T) []T {
	return interleavePartitions4WithSeed(0, partition)
}

// Use rand library for random number generation.
func interleavePartitions4WithSeed[T any](seed uint64, partition [][]T) []T {
	numTransactions := 0
	// This loop is O(|partition|) = O(|transactions|).
	for _, part := range partition {
		numTransactions += len(part)
	}
	if numTransactions == 0 {
		return nil
	}

	idxs := rand.Perm(numTransactions)

	type proxy struct {
		pos  int
		part int
	}
	proxies := make([]proxy, 0, numTransactions)

	// This loop nest is O(|transactions|).
	for i, part := range partition {
		for range len(part) {
			proxies = append(proxies, proxy{idxs[i], i})
		}
	}

	slices.SortFunc(proxies, func(a, b proxy) int {
		return cmp.Compare(a.pos, b.pos)
	})

	res := make([]T, 0, numTransactions)
	// This loop is O(|transactions|).
	for _, proxy := range proxies {
		res = append(res, partition[proxy.part][0])
		partition[proxy.part] = partition[proxy.part][1:]
	}
	return res
}

type xorShiftStar struct {
	state uint64
}

// seed initializes the xorShiftStar random number generator with the given seed.
// seed must be non-zero.
func (x *xorShiftStar) seed(seed uint64) {
	x.state = seed
}

// next returns the next random number using the xorshift* algorithm.
// Based on https://en.wikipedia.org/wiki/Xorshift#xorshift*
func (x *xorShiftStar) next() uint64 {
	const factor = uint64(0x2545F4914F6CDD1D)
	rand := x.state
	rand ^= rand >> 12
	rand ^= rand << 25
	rand ^= rand >> 27
	x.state = rand
	return rand * factor
}

// randN returns a random number in the range [0, n) using the xorshift* algorithm.
// it removes the modulo bias
func (x *xorShiftStar) randN(n uint64) uint64 {
	// remove modulo bias
	uint64Max := ^uint64(0)
	var rand uint64
	for {
		rand = x.next()
		// for small n, this will most likely be true in the first iteration
		if rand < (uint64Max - (uint64Max % n)) {
			break
		}
	}
	return rand % n
}
