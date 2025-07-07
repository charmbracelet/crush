//go:build go1.13
// +build go1.13

#include "textflag.h"

TEXT libc_closedir_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_closedir(SB)

GLOBL	·libc_closedir_trampoline_addr(SB), RODATA, $8
DATA	·libc_closedir_trampoline_addr(SB)/8, $libc_closedir_trampoline<>(SB)

TEXT libc_readdir_r_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_readdir_r(SB)

GLOBL	·libc_readdir_r_trampoline_addr(SB), RODATA, $8
DATA	·libc_readdir_r_trampoline_addr(SB)/8, $libc_readdir_r_trampoline<>(SB)

TEXT libc_opendir_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_opendir(SB)

GLOBL	·libc_opendir_trampoline_addr(SB), RODATA, $8
DATA	·libc_opendir_trampoline_addr(SB)/8, $libc_opendir_trampoline<>(SB)
