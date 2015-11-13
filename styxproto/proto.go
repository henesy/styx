package styxproto

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// This package does not unmarshal messages into structures.
// Instead, they are kept as-is and fields are parsed on-demand
// via methods. The msg helper type is used to access fields
// common to all 9P messages.
type msg []byte

func (m msg) Type() uint8   { return m[4] }
func (m msg) Tag() uint16   { return guint16(m[5:7]) }
func (m msg) Body() []byte  { return m[7:] }
func (m msg) nbytes() int64 { return int64(len(m)) }

// Each 9P message begins with a four-byte size field specifying the
// length in bytes of the complete message including the four bytes
// of the size field itself.
func (m msg) Len() int64 { return int64(guint32(m[:4])) }

// Calling nthField on a message that has not been verified
// can result in a run-time panic if the size headers are
// incorrect.
func (m msg) nthField(offset, n int) []byte {
	size := int(guint16(m[offset : offset+2]))
	for i := 0; i < n; i++ {
		offset += size + 2
		size = int(guint16(m[offset : offset+2]))
	}
	return m[offset+2 : offset+2+size]
}

// A Msg is a 9P message. 9P messages are sent by clients (T-messages)
// and servers (R-messages).
type Msg interface {
	// Tag is a transaction identifier. No two pending T-messages may
	// use the same tag. All R-messages must reference the T-message
	// being answered by using the same tag.
	Tag() uint16

	// Len returns the total length of the message in bytes.
	Len() int64

	// The actual number of bytes that the message takes up in
	// memory. For large Twrite/Rread messages, this is just the headers,
	// and not the payload.
	nbytes() int64
}

// The version request negotiates the protocol version and message
// size to be used on the connection and initializes the connection
// for I/O.  Tversion must be the first message sent on the 9P connection,
// and the client cannot issue any further requests until it has
// received the Rversion reply.
type Tversion []byte

// For version messages, Tag should be styx.NOTAG
func (m Tversion) Tag() uint16 { return msg(m).Tag() }

// Len returns the length of a Tversion request in bytes.
func (m Tversion) Len() int64    { return msg(m).Len() }
func (m Tversion) nbytes() int64 { return msg(m).nbytes() }

// Msize returns the maximum length, in bytes, that the client will
// ever generate or expect to receive in a single 9P message. This
// count includes all 9P protocol data, starting from the size field
// and extending through the message, but excludes enveloping transport
// protocols.
func (m Tversion) Msize() int64 { return int64(guint32(m[7:11])) }

// Version identifies the level of the protocol that the client supports.
// The string must always begin with the two characters "9P".
func (m Tversion) Version() []byte { return msg(m).nthField(11, 0) }
func (m Tversion) String() string {
	return fmt.Sprintf("Tversion msize=%d version=%q", m.Msize(), m.Version())
}

// An Rversion reply is sent in response to a Tversion request.
// It contains the version of the protocol that the server has
// chosen, and the maximum size of all successive messages.
type Rversion []byte

// Tag must return the tag of the corresponding Tversion message,
// NOTAG.
func (m Rversion) Tag() uint16 { return msg(m).Tag() }

// Len returns the length of the Rversion message in bytes.
func (m Rversion) Len() int64    { return msg(m).Len() }
func (m Rversion) nbytes() int64 { return msg(m).nbytes() }

// Version identifies the level of the protocol that the server supports. If a server
// does not understand the protocol version sent in a Tversion message, Version
// will return the string "unknown". A server may choose to specify a version that
// is less than or equal to that supported by the client.
func (m Rversion) Version() []byte { return Tversion(m).Version() }

// Msize returns the maximum size (in bytes) of any 9P message that
// it will send or accept, and must be equal to or less than the maximum
// suggested in the preceding Tversion message. After the Rversion
// message is received, both sides of the connection must honor this
// limit.
func (m Rversion) Msize() int64 { return Tversion(m).Msize() }

func (m Rversion) String() string {
	return fmt.Sprintf("Rversion msize=%d version=%q", m.Msize(), m.Version())
}

// The Tauth message is used to authenticate users on a connection.
type Tauth []byte

func (m Tauth) Tag() uint16   { return msg(m).Tag() }
func (m Tauth) Len() int64    { return msg(m).Len() }
func (m Tauth) nbytes() int64 { return msg(m).nbytes() }

// The afid of a Tversion message establishes an 'authentication file';
// after a Tauth message is accepted by the server, a client must carry
// out the authentication protocol by performing I/O operations on
// afid. Any protocol may be used and authentication is outside the
// scope of the 9P protocol.
func (m Tauth) Afid() uint32 { return guint32(m[7:11]) }

// The uname field contains the name of the user to authenticate.
func (m Tauth) Uname() []byte { return msg(m).nthField(11, 0) }

// The aname field contains the name of the file tree to access. It
// may be empty.
func (m Tauth) Aname() []byte { return msg(m).nthField(11, 1) }

func (m Tauth) String() string {
	return fmt.Sprintf("Tauth afid=%x uname=%q aname=%q", m.Afid(), m.Uname(), m.Aname())
}

// Servers that require authentication will reply to Tauth requests
// with an Rauth message. If a server does not require authentication,
// it can reply to a Tauth message with an Rerror message.
type Rauth []byte

func (m Rauth) Tag() uint16   { return msg(m).Tag() }
func (m Rauth) Len() int64    { return msg(m).Len() }
func (m Rauth) nbytes() int64 { return msg(m).nbytes() }

// The aqid of an Rauth message must be of type QTAUTH.
func (m Rauth) Aqid() Qid      { return Qid(m[7:20]) }
func (m Rauth) String() string { return fmt.Sprintf("Rauth aqid=%q", m.Aqid()) }

// The attach message serves as a fresh introduction from a  user on
// the client machine to the server.
type Tattach []byte

func (m Tattach) Tag() uint16   { return msg(m).Tag() }
func (m Tattach) Len() int64    { return msg(m).Len() }
func (m Tattach) nbytes() int64 { return msg(m).nbytes() }

// Fid establishes a fid to be used as the root of the file tree, should
// the client's Tattach request be accepted.
func (m Tattach) Fid() uint32 { return guint32(m[7:11]) }

// On servers that require authentication, afid serves to authenticate a user,
// and must have been established in a previous Tauth request. If a client
// does not wish to authenticate, afid should be set to styx.NOFID.
func (m Tattach) Afid() uint32 { return guint32(m[11:15]) }

// Uname is the user name of the attaching user.
func (m Tattach) Uname() []byte { return msg(m).nthField(15, 0) }

// Aname is the name of the file tree that the client wants to access.
func (m Tattach) Aname() []byte { return msg(m).nthField(15, 1) }

func (m Tattach) String() string {
	return fmt.Sprintf("Tattach fid=%x afid=%x uname=%q aname=%q",
		m.Fid(), m.Afid(), m.Uname(), m.Aname())
}

type Rattach []byte

func (m Rattach) Tag() uint16   { return msg(m).Tag() }
func (m Rattach) Len() int64    { return msg(m).Len() }
func (m Rattach) nbytes() int64 { return msg(m).nbytes() }

// Qid is the qid of the root of the file tree. Qid is associated
// with the fid of the corresponding Tattach request.
func (m Rattach) Qid() Qid       { return Qid(m[7:20]) }
func (m Rattach) String() string { return fmt.Sprintf("Rattach qid=%q", m.Qid()) }

type Rerror []byte

func (m Rerror) Tag() uint16   { return msg(m).Tag() }
func (m Rerror) Len() int64    { return msg(m).Len() }
func (m Rerror) nbytes() int64 { return msg(m).nbytes() }

// Ename is a UTF-8 string describing the error that occured.
func (m Rerror) Ename() []byte { return msg(m).nthField(7, 0) }

// The Error method implements the error interface, and
// returns the result of the Ename method as a string.
func (m Rerror) Error() string { return string(m.Ename()) }

func (m Rerror) String() string { return fmt.Sprintf("Rerror ename=%q", m.Ename()) }

type Tflush []byte

func (m Tflush) Tag() uint16    { return msg(m).Tag() }
func (m Tflush) Len() int64     { return msg(m).Len() }
func (m Tflush) nbytes() int64  { return msg(m).nbytes() }
func (m Tflush) Oldtag() uint16 { return guint16(m[7:9]) }
func (m Tflush) String() string { return fmt.Sprintf("Tflush oldtag=%x", m.Oldtag()) }

type Rflush []byte

func (m Rflush) Tag() uint16    { return msg(m).Tag() }
func (m Rflush) Len() int64     { return msg(m).Len() }
func (m Rflush) nbytes() int64  { return msg(m).nbytes() }
func (m Rflush) String() string { return "Rflush" }

type Twalk []byte

func (m Twalk) Tag() uint16        { return msg(m).Tag() }
func (m Twalk) Len() int64         { return msg(m).Len() }
func (m Twalk) nbytes() int64      { return msg(m).nbytes() }
func (m Twalk) Fid() uint32        { return guint32(m[7:11]) }
func (m Twalk) Newfid() uint32     { return guint32(m[11:15]) }
func (m Twalk) Nwname() int        { return int(guint16(m[15:17])) }
func (m Twalk) Wname(n int) []byte { return msg(m).nthField(17, n) }
func (m Twalk) String() string {
	var buf [MaxWElem][]byte
	wname := buf[:0]

	for i := 0; i < m.Nwname(); i++ {
		wname = append(wname, m.Wname(i))
	}
	path := bytes.Join(wname, []byte("/"))
	return fmt.Sprintf("Twalk fid=%x newfid=%x %q", m.Fid(), m.Newfid(), path)
}

type Rwalk []byte

func (m Rwalk) Tag() uint16   { return msg(m).Tag() }
func (m Rwalk) Len() int64    { return msg(m).Len() }
func (m Rwalk) nbytes() int64 { return msg(m).nbytes() }

// Nwqid must always be equal to or lesser than Nwname of the corresponding
// Twalk request. Only if Nwqid is equal to Nwname is the Newfid of
// the Twalk request established. Nwqid must always be greater than
// zero.
func (m Rwalk) Nwqid() int { return int(guint16(m[7:9])) }

// Wqid contains the Qid values of each path in the walk
// requested by the client, up to the first failure.
func (m Rwalk) Wqid(n int) Qid { return Qid(m[9+n*13 : 9+(n+1)*13]) }

func (m Rwalk) String() string {
	var buf [MaxWElem]string
	wqid := buf[:0]

	for i := 0; i < m.Nwqid(); i++ {
		wqid = append(wqid, m.Wqid(i).String())
	}
	return fmt.Sprintf("Rwalk wqid=%q", strings.Join(wqid, ","))
}

type Topen []byte

func (m Topen) Tag() uint16   { return msg(m).Tag() }
func (m Topen) Len() int64    { return msg(m).Len() }
func (m Topen) nbytes() int64 { return msg(m).nbytes() }
func (m Topen) Fid() uint32   { return guint32(m[7:11]) }
func (m Topen) Mode() uint8   { return uint8(m[11]) }
func (m Topen) String() string {
	return fmt.Sprintf("Topen fid=%x mode=%#o", m.Fid(), m.Mode())
}

type Ropen []byte

func (m Ropen) Tag() uint16   { return msg(m).Tag() }
func (m Ropen) Len() int64    { return msg(m).Len() }
func (m Ropen) nbytes() int64 { return msg(m).nbytes() }

// Qid contains the unique identifier of the opened file.
func (m Ropen) Qid() Qid { return Qid(m[7:20]) }

// The iounit field returned by open and create may be zero.  If it
// is not, it is the maximum number of bytes that are guaranteed to
// be read from or written to the file without breaking the I/O transfer
// into multiple 9P messages
func (m Ropen) IOunit() int64 { return int64(guint32(m[20:24])) }
func (m Ropen) String() string {
	return fmt.Sprintf("Ropen qid=%q iounit=%d", m.Qid(), m.IOunit())
}

type Tcreate []byte

func (m Tcreate) Tag() uint16   { return msg(m).Tag() }
func (m Tcreate) Len() int64    { return msg(m).Len() }
func (m Tcreate) nbytes() int64 { return msg(m).nbytes() }
func (m Tcreate) Fid() uint32   { return guint32(m[7:11]) }
func (m Tcreate) Name() []byte  { return msg(m).nthField(11, 0) }
func (m Tcreate) Perm() uint32 {
	// Would have been nice if Name came at the end
	offset := 11 + 2 + guint16(m[11:13])
	return guint32(m[offset : offset+4])
}
func (m Tcreate) Mode() uint8 { return m[len(m.Name())+17] }
func (m Tcreate) String() string {
	return fmt.Sprintf("Tcreate fid=%x name=%q perm=%o mode=%#o",
		m.Fid(), m.Name(), m.Perm(), m.Mode())
}

type Rcreate []byte

func (m Rcreate) Tag() uint16   { return msg(m).Tag() }
func (m Rcreate) Len() int64    { return msg(m).Len() }
func (m Rcreate) nbytes() int64 { return msg(m).nbytes() }
func (m Rcreate) Qid() Qid      { return Qid(m[7:20]) }
func (m Rcreate) IOunit() int64 { return int64(guint32(m[20:24])) }
func (m Rcreate) String() string {
	return fmt.Sprintf("Rcreate qid=%q iounit=%d", m.Qid(), m.IOunit())
}

type Tread []byte

func (m Tread) Tag() uint16   { return msg(m).Tag() }
func (m Tread) Len() int64    { return msg(m).Len() }
func (m Tread) nbytes() int64 { return msg(m).nbytes() }
func (m Tread) Fid() uint32   { return guint32(m[7:11]) }
func (m Tread) Offset() int64 { return int64(guint64(m[11:19])) }
func (m Tread) Count() int64  { return int64(guint32(m[19:23])) }
func (m Tread) String() string {
	return fmt.Sprintf("Tread offset=%d count=%d", m.Offset(), m.Count())
}

// The Rread message returns the bytes requested by a Tread message.
// The data portion of an Rread message can be consumed using the io.Reader
// interface.
type Rread struct {
	io.Reader
	msg msg // headers plus any extra buffered data
}

// If a Tread requests asks for more data than can fit within a single 9P
// message, multiple Rread messages will be generated that cite the tag
// of a single Tread request.
func (m Rread) Tag() uint16    { return m.msg.Tag() }
func (m Rread) Len() int64     { return m.msg.Len() }
func (m Rread) nbytes() int64  { return m.msg.nbytes() }
func (m Rread) Count() int64   { return int64(guint32(m.msg[7:11])) }
func (m Rread) String() string { return fmt.Sprintf("Rread count=%d", m.Count()) }

// The Twrite message is sent by a client to write data to a file.
// The data portion of a Twrite request can be accessed via the
// io.Reader interface.
type Twrite struct {
	io.Reader
	msg msg // headers plus any extra buffered data
}

func (m Twrite) Tag() uint16   { return m.msg.Tag() }
func (m Twrite) Len() int64    { return m.msg.Len() }
func (m Twrite) nbytes() int64 { return m.msg.nbytes() }
func (m Twrite) Fid() uint32   { return Tread(m.msg).Fid() }
func (m Twrite) Offset() int64 { return Tread(m.msg).Offset() }
func (m Twrite) Count() int64  { return Tread(m.msg).Count() }

func (m Twrite) String() string {
	return fmt.Sprintf("Twrite fid=%x offset=%d count=%d",
		m.Fid(), m.Offset(), m.Count())
}

type Rwrite []byte

func (m Rwrite) Tag() uint16    { return msg(m).Tag() }
func (m Rwrite) Len() int64     { return msg(m).Len() }
func (m Rwrite) nbytes() int64  { return msg(m).nbytes() }
func (m Rwrite) Count() uint32  { return guint32(m[7:11]) }
func (m Rwrite) String() string { return fmt.Sprintf("Rwrite count=%d", m.Count()) }

type Tclunk []byte

func (m Tclunk) Tag() uint16    { return msg(m).Tag() }
func (m Tclunk) Len() int64     { return msg(m).Len() }
func (m Tclunk) nbytes() int64  { return msg(m).nbytes() }
func (m Tclunk) Fid() uint32    { return guint32(m[4:11]) }
func (m Tclunk) String() string { return fmt.Sprintf("Tclunk fid=%x", m.Fid()) }

type Rclunk []byte

func (m Rclunk) Tag() uint16    { return msg(m).Tag() }
func (m Rclunk) Len() int64     { return msg(m).Len() }
func (m Rclunk) nbytes() int64  { return msg(m).nbytes() }
func (m Rclunk) String() string { return "Rclunk" }

type Tremove []byte

func (m Tremove) Tag() uint16    { return msg(m).Tag() }
func (m Tremove) Len() int64     { return msg(m).Len() }
func (m Tremove) nbytes() int64  { return msg(m).nbytes() }
func (m Tremove) Fid() uint32    { return guint32(m[4:11]) }
func (m Tremove) String() string { return fmt.Sprintf("Tremove fid=%x", m.Fid()) }

type Rremove []byte

func (m Rremove) Tag() uint16    { return msg(m).Tag() }
func (m Rremove) Len() int64     { return msg(m).Len() }
func (m Rremove) nbytes() int64  { return msg(m).nbytes() }
func (m Rremove) String() string { return "Rremove" }

type Tstat []byte

func (m Tstat) Tag() uint16    { return msg(m).Tag() }
func (m Tstat) Len() int64     { return msg(m).Len() }
func (m Tstat) nbytes() int64  { return msg(m).nbytes() }
func (m Tstat) Fid() uint32    { return guint32(m[7:11]) }
func (m Tstat) String() string { return fmt.Sprintf("Tstat fid=%x", m.Fid()) }

type Rstat []byte

func (m Rstat) Tag() uint16    { return msg(m).Tag() }
func (m Rstat) Len() int64     { return msg(m).Len() }
func (m Rstat) nbytes() int64  { return msg(m).nbytes() }
func (m Rstat) Stat() Stat     { return msg(m).nthField(7, 0) }
func (m Rstat) String() string { return "Rstat " + m.Stat().String() }

type Twstat []byte

func (m Twstat) Tag() uint16    { return msg(m).Tag() }
func (m Twstat) Len() int64     { return msg(m).Len() }
func (m Twstat) nbytes() int64  { return msg(m).nbytes() }
func (m Twstat) Fid() uint32    { return guint32(m[7:11]) }
func (m Twstat) Stat() Stat     { return msg(m).nthField(7, 0) }
func (m Twstat) String() string { return fmt.Sprintf("Twstat fid=%x stat=%q", m.Fid(), m.Stat()) }

type Rwstat []byte

func (m Rwstat) Tag() uint16    { return msg(m).Tag() }
func (m Rwstat) Len() int64     { return msg(m).Len() }
func (m Rwstat) nbytes() int64  { return msg(m).nbytes() }
func (m Rwstat) String() string { return "Rwstat" }

// BadMessage represents an invalid message.
type BadMessage struct {
	Err    error // the reason the message is invalid
	length int64 // the message bytes
	tag    uint16
}

// Tag returns the tag of the errant message. Servers
// should cite the same tag when replying with an Rerror
// message.
func (m BadMessage) Tag() uint16    { return m.tag }
func (m BadMessage) Len() int64     { return m.length }
func (m BadMessage) nbytes() int64  { return m.length }
func (m BadMessage) String() string { return fmt.Sprintf("bad message: %v", m.Err) }
