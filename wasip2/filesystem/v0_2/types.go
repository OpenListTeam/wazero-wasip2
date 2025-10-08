package v0_2

import (
	clocks_v0_2 "github.com/foxxorcat/wazero-wasip2/wasip2/clocks/v0_2"
	io_v0_2 "github.com/foxxorcat/wazero-wasip2/wasip2/io/v0_2"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"
)

// --- Imported Types ---
type InputStream = io_v0_2.InputStream
type OutputStream = io_v0_2.OutputStream
type WasiError = io_v0_2.Error
type Datetime = clocks_v0_2.Datetime

// --- Base Types ---
type Filesize = uint64
type LinkCount = uint64
type Descriptor = uint32
type DirectoryEntryStream = uint32

// --- Enums and Flags ---

type DescriptorType uint8

const (
	DescriptorTypeUnknown         DescriptorType = iota // 未知类型
	DescriptorTypeBlockDevice                           // 块设备
	DescriptorTypeCharacterDevice                       // 字符设备
	DescriptorTypeDirectory                             // 目录
	DescriptorTypeFifo                                  // 命名管道 (FIFO)
	DescriptorTypeSymbolicLink                          // 符号链接
	DescriptorTypeRegularFile                           // 常规文件
	DescriptorTypeSocket                                // 套接字
)

type DescriptorFlags struct {
	Read               bool
	Write              bool
	FileIntegritySync  bool
	DataIntegritySync  bool
	RequestedWriteSync bool
	MutateDirectory    bool
}

func (DescriptorFlags) IsFlags() {}

type PathFlags struct {
	SymlinkFollow bool
}

func (PathFlags) IsFlags() {}

type OpenFlags struct {
	Create    bool
	Directory bool
	Exclusive bool
	Truncate  bool
}

func (OpenFlags) IsFlags() {}

// Advice 定义了文件或内存访问模式的建议信息。
type Advice uint8

const (
	AdviceNormal     Advice = iota // 无特殊建议。
	AdviceSequential               // 预期顺序访问。
	AdviceRandom                   // 预期随机访问。
	AdviceWillNeed                 // 预期很快会访问。
	AdviceDontNeed                 // 预期短期内不会访问。
	AdviceNoReuse                  // 预期只会访问一次。
)

type ErrorCode uint8

const (
	ErrorCodeAccess              ErrorCode = iota // 权限被拒绝 (EACCES)
	ErrorCodeWouldBlock                           // 资源暂时不可用或操作会阻塞 (EAGAIN / EWOULDBLOCK)
	ErrorCodeAlready                              // 连接已在进行中 (EALREADY)
	ErrorCodeBadDescriptor                        // 错误的描述符 (EBADF)
	ErrorCodeBusy                                 // 设备或资源忙 (EBUSY)
	ErrorCodeDeadlock                             // 会发生资源死锁 (EDEADLK)
	ErrorCodeQuota                                // 超出存储配额 (EDQUOT)
	ErrorCodeExist                                // 文件已存在 (EEXIST)
	ErrorCodeFileTooLarge                         // 文件过大 (EFBIG)
	ErrorCodeIllegalByteSequence                  // 非法的字节序列 (EILSEQ)
	ErrorCodeInProgress                           // 操作正在进行中 (EINPROGRESS)
	ErrorCodeInterrupted                          // 函数被中断 (EINTR)
	ErrorCodeInvalid                              // 无效的参数 (EINVAL)
	ErrorCodeIo                                   // I/O 错误 (EIO)
	ErrorCodeIsDirectory                          // 是一个目录 (EISDIR)
	ErrorCodeLoop                                 // 检测到过多的符号链接层级 (ELOOP)
	ErrorCodeTooManyLinks                         // 链接过多 (EMLINK)
	ErrorCodeMessageSize                          // 消息过长 (EMSGSIZE)
	ErrorCodeNameTooLong                          // 文件名过长 (ENAMETOOLONG)
	ErrorCodeNoDevice                             // 没有该设备 (ENODEV)
	ErrorCodeNoEntry                              // 没有该文件或目录 (ENOENT)
	ErrorCodeNoLock                               // 没有可用的锁 (ENOLCK)
	ErrorCodeInsufficientMemory                   // 内存不足 (ENOMEM)
	ErrorCodeInsufficientSpace                    // 设备上没有剩余空间 (ENOSPC)
	ErrorCodeNotDirectory                         // 不是一个目录 (ENOTDIR)
	ErrorCodeNotEmpty                             // 目录不为空 (ENOTEMPTY)
	ErrorCodeNotRecoverable                       // 状态不可恢复 (ENOTRECOVERABLE)
	ErrorCodeUnsupported                          // 不支持的操作 (ENOTSUP / ENOSYS)
	ErrorCodeNoTty                                // 不是一个终端设备 (ENOTTY)
	ErrorCodeNoSuchDevice                         // 没有该设备或地址 (ENXIO)
	ErrorCodeOverflow                             // 值对于数据类型来说过大 (EOVERFLOW)
	ErrorCodeNotPermitted                         // 操作不被允许 (EPERM)
	ErrorCodePipe                                 // 管道中断 (EPIPE)
	ErrorCodeReadOnly                             // 只读文件系统 (EROFS)
	ErrorCodeInvalidSeek                          // 无效的 seek 操作 (ESPIPE)
	ErrorCodeTextFileBusy                         // 文本文件忙 (ETXTBSY)
	ErrorCodeCrossDevice                          // 跨设备链接 (EXDEV)
)

// --- Records and Variants ---

type DescriptorStat struct {
	Type                      DescriptorType
	LinkCount                 LinkCount
	Size                      Filesize
	DataAccessTimestamp       witgo.Option[Datetime]
	DataModificationTimestamp witgo.Option[Datetime]
	StatusChangeTimestamp     witgo.Option[Datetime]
}

type NewTimestamp struct {
	NoChange  *witgo.Unit `wit:"case(0)"`
	Now       *witgo.Unit `wit:"case(1)"`
	Timestamp *Datetime   `wit:"case(2)"`
}

type DirectoryEntry struct {
	Type DescriptorType
	Name string
}

// MetadataHashValue 是一个128位的哈希值。
type MetadataHashValue struct {
	Lower uint64 // 哈希值的低64位
	Upper uint64 // 哈希值的高64位
}
