package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"bytes"
	"labgob"
	"log"
	"math/rand"
	"sync"
	"time"
)
import "labrpc"

// import "bytes"
// import "labgob"

const broadcastTime = time.Duration(100 * time.Millisecond)
const electionTimeout = time.Duration(1000 * time.Millisecond)

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

type LogEntry struct {
	LogIndex	int
	Command 	interface{}
	Term 		int
}

type serverState int
const (
	Follower serverState = iota
	Candidate
	Leader
)

type Err string
const (
	OK = 	"OK"
	RPCFAIL =	"RPCFAIL"
)

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	currentLeader 	int
	state 			serverState
	currentTerm		int
	logIndex		int //index of next log entry to be stored, initialized to 1
	votedFor 		int
	log 			[]LogEntry

	commitIndex		int
	lastApplied		int

	//每次选举成功nextIndex都重新初始化为logIndex，所以Leader的nextIndex总是>=follower的logIndex
	nextIndex		[]int
	matchIndex		[]int
	notifyApplyMsg	chan struct{} //更新commitIndex时chan写入
	shutdown		chan struct{}
	electionTimer	*time.Timer
	testFlag		bool
}

func generateRandDuration(minDuration time.Duration) time.Duration {
	extra := time.Duration(rand.Int63()) % minDuration
	return time.Duration(minDuration + extra)
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.currentTerm, rf.state == Leader
}

func (rf *Raft) resetElectionTimer(duration time.Duration) {
	//if !rf.electionTimer.Stop() {
	//	<-rf.electionTimer.C
	//}
	//rf.electionTimer.Reset(duration)
	//todo 这样设置是否合适?
	rf.electionTimer.Stop()
	rf.electionTimer.Reset(duration)
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
//dump
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)

	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.log)
	e.Encode(rf.logIndex)
	e.Encode(rf.commitIndex)
	e.Encode(rf.lastApplied)
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
//load
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }

	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	currentTerm, votedFor, logIndex, commitIndex, lastApplied := 0, 0, 0, 0, 0
	//n1, n2, n3, n4, n5, n6 := d.Decode(&currentTerm), d.Decode(&votedFor), d.Decode(&logIndex),
	//	d.Decode(&commitIndex), d.Decode(&lastApplied), d.Decode(&rf.log)
	//DPrintf("%v, %v, %v, %v, %v, %v", n1, n2, n3, n4, n5, n6)
	if d.Decode(&currentTerm) != nil ||
		d.Decode(&votedFor) != nil ||
		d.Decode(&rf.log) != nil ||
		d.Decode(&logIndex) != nil ||
		d.Decode(&commitIndex) != nil ||
		d.Decode(&lastApplied) != nil {
		//DPrintf("%v, %v, %v, %v, %v, %v", currentTerm, votedFor, logIndex, commitIndex, lastApplied, rf.log)
		log.Fatal("error while unmarshal raft state.")
	}
	rf.currentTerm, rf.votedFor, rf.logIndex, rf.commitIndex, rf.lastApplied =
		currentTerm, votedFor, logIndex, commitIndex, lastApplied
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	//DPrintf("follower rf: %v", *rf)
	if rf.state != Leader {
		return -1, -1, false
	}

	rf.testFlag = true
	index := rf.logIndex
	term := rf.currentTerm
	entry := LogEntry{LogIndex:index, Term:term, Command:command}
	rf.log = append(rf.log, entry)
	rf.matchIndex[rf.me] = index
	rf.logIndex += 1
	rf.persist()
	DPrintf("currentLeader: %v, me: %v", rf.currentLeader, rf.me)
	go rf.replicate()

	return index, term, true
}

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	// Your code here, if desired.
}

func (rf *Raft) solicit(i int, args *RequestVoteArgs, replyCh chan<- RequestVoteReply) {
	reply := RequestVoteReply{}
	if rf.peers[i].Call("Raft.RequestVote", args, &reply) == false {
		reply.Err = RPCFAIL
		reply.Server = i
	}
	replyCh <- reply
}

func (rf *Raft) reinitIndex() {
	peersNum := len(rf.peers)
	rf.nextIndex, rf.matchIndex = make([]int, peersNum), make([]int, peersNum)
	for i := 0; i < peersNum; i++ {
		if rf.logIndex == 0 {
			DPrintf("find 0")
		}
		rf.nextIndex[i] = rf.logIndex
		rf.matchIndex[i] = 0
	}
}

func (rf *Raft) stepDown(term int) {
	rf.currentTerm = term
	rf.state = Follower
	rf.votedFor = -1
	rf.persist()
	rf.resetElectionTimer(generateRandDuration(electionTimeout))
}

func (rf *Raft) tick() {
	timer := time.NewTimer(broadcastTime)
	for {
		select {
		case <-timer.C:
			if _, isLeader := rf.GetState(); !isLeader {
				return
			}
			go rf.replicate()
			timer.Stop()
			timer.Reset(broadcastTime)
		case <-rf.shutdown:
			return
		}
	}
}

func (rf *Raft) replicate() {
	for follower := 0; follower < len(rf.peers); follower++ {
		if follower != rf.me {
			go rf.sendLogEntry(follower)
		}
	}
}

func (rf *Raft) sendLogEntry(follower int) {
	rf.mu.Lock()
	if rf.state != Leader {
		rf.mu.Unlock()
		return
	}
	args := AppendEntriesArgs{}
	args.Term = rf.currentTerm
	args.LeaderId = rf.me
	prevLogIndex := rf.nextIndex[follower] - 1
	//DPrintf("follower: %v, rf.nextIndex: %v, prevLogIndex: %v, loglen: %v",
	//	follower, rf.nextIndex, prevLogIndex, len(rf.log))
	args.PrevLogIndex = prevLogIndex
	args.PrevLogTerm = rf.log[prevLogIndex].Term
	args.LeaderCommit = rf.commitIndex
	if rf.logIndex > rf.nextIndex[follower] {
		entries := rf.log[prevLogIndex+1:rf.logIndex]
		args.Entries = entries
	} else {
		args.Entries = nil
	}
	rf.mu.Unlock()

	var reply AppendEntriesReply
	//不用理会失败情况，follower接收不到AppendEntries超时会发起选举
	if rf.peers[follower].Call("Raft.AppendEntries", &args, &reply) {
		rf.mu.Lock()
		if !reply.Success {
			if reply.Term > rf.currentTerm {
				rf.stepDown(reply.Term)
			} else {
				//retry
				rf.nextIndex[follower] = Max(1, reply.ConflictIndex)
				//DPrintf("retry rf.nextIndex: %v follower: %v confilict: %v",
				//	rf.nextIndex, follower, reply.ConflictIndex)
				go rf.sendLogEntry(follower)
			}
		} else {
			entriesLen := 0
			if args.Entries != nil {
				entriesLen = len(args.Entries)
			}
			commitIndex := prevLogIndex + entriesLen
			rf.nextIndex[follower] = commitIndex + 1
			rf.matchIndex[follower] = commitIndex

			if rf.canCommit(commitIndex) {
				rf.commitIndex = commitIndex
				rf.persist()
				rf.notifyApplyMsg <- struct{}{}
			}
		}
		rf.mu.Unlock()
	}
	//DPrintf("me: %v, leader: %v", rf.me, rf.currentLeader)
}

func (rf *Raft) canCommit(index int) bool {
	if index < rf.logIndex && index > rf.commitIndex && rf.log[index].Term == rf.currentTerm {
		majorities, count := len(rf.peers) / 2 + 1, 0
		for i := 0; i < len(rf.peers); i++ {
			if rf.matchIndex[i] >= index {
				count += 1
			}
		}
		return count >= majorities
	} else {
		return false
	}
}

func (rf *Raft) campaign() {
	//DPrintf("%v start campaign", rf.me)
	rf.mu.Lock()
	if rf.state == Leader {
		rf.mu.Unlock()
		return
	}

	rf.state = Candidate
	rf.currentTerm += 1
	rf.votedFor = rf.me
	//generateRandDuration(electionTimeout) always greater than electionTimeout
	timer := time.After(electionTimeout)
	//DPrintf("%v start campaign. current term: %v", rf.me, rf.currentTerm)

	rf.resetElectionTimer(generateRandDuration(electionTimeout))
	args := RequestVoteArgs{}
	args.Term = rf.currentTerm
	args.CandidateId = rf.me
	args.LastLogIndex = rf.logIndex - 1
	args.LastLogTerm = rf.log[args.LastLogIndex].Term
	rf.persist()
	rf.mu.Unlock()

	replyCh := make(chan RequestVoteReply, len(rf.peers) - 1)
	for i := 0; i < len(rf.peers); i++ {
		if i != rf.me {
			go rf.solicit(i, &args, replyCh)
		}
	}

	voteCount, majorityCount := 0, len(rf.peers)/2
	for voteCount < majorityCount {
		select {
		case <-timer:
			return
		case reply := <-replyCh:
			if reply.VoteGranted {
				voteCount += 1
			} else if reply.Err != OK {
				// retry
				go rf.solicit(reply.Server, &args, replyCh)
			} else {
				rf.mu.Lock()
				//出现network partition，产生了新的term
				if reply.Term > rf.currentTerm {
					//发现term不是最新的，stepdown后停止竞选
					rf.stepDown(reply.Term)
					rf.mu.Unlock()
					return
				}
				rf.mu.Unlock()
			}
		}
	}
	//DPrintf("server %d win the election, current term: %v", rf.me, rf.currentTerm)

	rf.mu.Lock()
	if rf.state == Candidate {
		rf.state = Leader
		rf.currentLeader = rf.me
		rf.reinitIndex()
		go rf.tick()
	}
	rf.mu.Unlock()
}

func (rf *Raft) apply(applyCh chan<- ApplyMsg) {
	for {
		select {
		case <-rf.notifyApplyMsg:
			//DPrintf("received notifyApplyMsg")
			rf.mu.Lock()
			var entries []LogEntry
			if rf.lastApplied < rf.commitIndex {
				entries = rf.log[rf.lastApplied+1:rf.commitIndex+1]
				rf.lastApplied = rf.commitIndex
			}
			rf.persist()
			rf.mu.Unlock()

			//DPrintf("entries: %v", entries)
			for _, entry := range entries {
				DPrintf("%v apply msg %v to applyCh", rf.me, entry)
				applyCh <- ApplyMsg{CommandValid: true, Command: entry.Command, CommandIndex: entry.LogIndex}
			}
		case <-rf.shutdown:
			return
		}
	}
}

func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.currentLeader = -1
	rf.state = Follower
	rf.currentTerm = 0
	rf.logIndex = 1
	rf.votedFor = -1
	rf.log = []LogEntry{{0, nil, 0}} //log entry at index 0 is unused
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.nextIndex = make([]int, 0)
	rf.matchIndex = make([]int, 0)
	rf.notifyApplyMsg = make(chan struct{}, 100)
	rf.shutdown = make(chan struct{})
	rf.testFlag = false
	rf.electionTimer = time.NewTimer(generateRandDuration(electionTimeout))

	// Your initialization code here (2A, 2B, 2C).

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	go rf.apply(applyCh)
	go func() {
		for {
			select {
			case <-rf.electionTimer.C:
				//DPrintf("%v start campaign", rf.me)
				rf.campaign()
			case <-rf.shutdown:
				return
			}
		}
	}()

	return rf
}
