VERSION = 1
PATCHLEVEL = 38
SUBLEVEL = 1
EXTRAVERSION = .git
NAME = Unnamed

# Colon is used as a separator in makefiles.  Strip any drive prefix
# from the current directory to avoid confusion.
CURDIR := $(lastword $(subst :, ,$(CURDIR)))

# *DOCUMENTATION*
# To see a list of typical targets execute "make help"
# More info can be located in ./README
# Comments in this file are targeted only to the developer, do not
# expect to learn how to build the kernel reading this file.

# Do not print "Entering directory ..."
MAKEFLAGS += --no-print-directory

# We are using a recursive build, so we need to do a little thinking
# to get the ordering right.
#
# Most importantly: sub-Makefiles should only ever modify files in
# their own directory. If in some directory we have a dependency on
# a file in another dir (which doesn't happen often, but it's often
# unavoidable when linking the built-in.o targets which finally
# turn into busybox), we will call a sub make in that other dir, and
# after that we are sure that everything which is in that other dir
# is now up to date.
#
# The only cases where we need to modify files which have global
# effects are thus separated out and done before the recursive
# descending is started. They are now explicitly listed as the
# prepare rule.

# To put more focus on warnings, be less verbose as default
# Use 'make V=1' to see the full commands

ifdef V
  ifeq ("$(origin V)", "command line")
    KBUILD_VERBOSE = $(V)
  endif
endif
ifndef KBUILD_VERBOSE
  KBUILD_VERBOSE = 0
endif

# kbuild supports saving output files in a separate directory.
# To locate output files in a separate directory two syntaxes are supported.
# In both cases the working directory must be the root of the kernel src.
# 1) O=
# Use "make O=dir/to/store/output/files/"
#
# 2) Set KBUILD_OUTPUT
# Set the environment variable KBUILD_OUTPUT to point to the directory
# where the output files shall be placed.
# export KBUILD_OUTPUT=dir/to/store/output/files/
# make
#
# The O= assignment takes precedence over the KBUILD_OUTPUT environment
# variable.


# KBUILD_SRC is set on invocation of make in OBJ directory
# KBUILD_SRC is not intended to be used by the regular user (for now)
ifeq ($(KBUILD_SRC),)

# OK, Make called in directory where kernel src resides
# Do we want to locate output files in a separate directory?
ifdef O
  ifeq ("$(origin O)", "command line")
    KBUILD_OUTPUT := $(O)
  endif
endif

# That's our default target when none is given on the command line
PHONY := _all
_all:

ifneq ($(KBUILD_OUTPUT),)
# Invoke a second make in the output directory, passing relevant variables
# check that the output directory actually exists
saved-output := $(KBUILD_OUTPUT)
KBUILD_OUTPUT := $(shell cd $(KBUILD_OUTPUT) && /bin/pwd)
$(if $(KBUILD_OUTPUT),, \
     $(error output directory "$(saved-output)" does not exist))

PHONY += $(MAKECMDGOALS)

$(filter-out _all,$(MAKECMDGOALS)) _all:
	$(if $(KBUILD_VERBOSE:1=),@)$(MAKE) -C $(KBUILD_OUTPUT) \
		KBUILD_SRC=$(CURDIR) -f $(CURDIR)/Makefile $@

# Leave processing to above invocation of make
skip-makefile := 1
endif # ifneq ($(KBUILD_OUTPUT),)
endif # ifeq ($(KBUILD_SRC),)

# We process the rest of the Makefile if this is the final invocation of make
ifeq ($(skip-makefile),)

PHONY += all
_all: all

srctree		:= $(if $(KBUILD_SRC),$(KBUILD_SRC),$(CURDIR))
TOPDIR		:= $(srctree)
# FIXME - TOPDIR is obsolete, use srctree/objtree
objtree		:= $(CURDIR)
src		:= $(srctree)
obj		:= $(objtree)

VPATH		:= $(srctree)

export srctree objtree VPATH TOPDIR


# Cross compiling and selecting different set of gcc/bin-utils
# ---------------------------------------------------------------------------
#
# When performing cross compilation for other architectures ARCH shall be set
# to the target architecture. (See arch/* for the possibilities).
# ARCH can be set during invocation of make:
# make ARCH=ia64
# Another way is to have ARCH set in the environment.
# The default ARCH is the host where make is executed.

# CROSS_COMPILE specify the prefix used for all executables used
# during compilation. Only gcc and related bin-utils executables
# are prefixed with $(CROSS_COMPILE).
# CROSS_COMPILE can be set on the command line
# make CROSS_COMPILE=ia64-linux-
# Alternatively CROSS_COMPILE can be set in the environment.
# Default value for CROSS_COMPILE is not to prefix executables
# Note: Some architectures assign CROSS_COMPILE in their arch/*/Makefile

CROSS_COMPILE ?=
# bbox: we may have CONFIG_CROSS_COMPILER_PREFIX in .config,
# and it has not been included yet... thus using an awkward syntax.
ifeq ($(CROSS_COMPILE),)
CROSS_COMPILE := $(shell grep ^CONFIG_CROSS_COMPILER_PREFIX .config 2>/dev/null)
CROSS_COMPILE := $(subst CONFIG_CROSS_COMPILER_PREFIX=,,$(CROSS_COMPILE))
CROSS_COMPILE := $(subst ",,$(CROSS_COMPILE))
#")
endif

# SUBARCH is the underlying arch, derived from CROSS_COMPILE (or from
# the host if not cross compiling).  ARCH defaults to SUBARCH and
# selects the arch/$(ARCH)/Makefile to include.

ifneq ($(CROSS_COMPILE),)
SUBARCH := $(shell echo $(CROSS_COMPILE) | cut -d- -f1 | sed 's:^.*/::g')
else
SUBARCH := $(shell uname -m)
endif
SUBARCH := $(shell echo $(SUBARCH) | sed -e s/i.86/i386/ -e s/sun4u/sparc64/ \
					 -e s/arm.*/arm/ -e s/sa110/arm/ \
					 -e s/s390x/s390/ -e s/parisc64/parisc/ \
					 -e s/ppc.*/powerpc/ -e s/mips.*/mips/ )

ARCH ?= $(SUBARCH)

HOST_COMPILER ?=
ifeq ($(HOST_COMPILER),)
HOST_COMPILER := $(shell grep ^CONFIG_HOST_COMPILER= .config 2>/dev/null)
HOST_COMPILER := $(subst CONFIG_HOST_COMPILER=,,$(HOST_COMPILER))
HOST_COMPILER := $(subst ",,$(HOST_COMPILER))
#")
endif
ifeq ($(HOST_COMPILER),)
HOST_COMPILER := gcc
endif

CROSS_COMPILER ?=
ifeq ($(CROSS_COMPILER),)
CROSS_COMPILER := $(shell grep ^CONFIG_CROSS_COMPILER= .config 2>/dev/null)
CROSS_COMPILER := $(subst CONFIG_CROSS_COMPILER=,,$(CROSS_COMPILER))
CROSS_COMPILER := $(subst ",,$(CROSS_COMPILER))
#")
endif
ifeq ($(CROSS_COMPILER),)
CROSS_COMPILER := gcc
endif

# SHELL used by kbuild
CONFIG_SHELL := $(shell if [ -x "$$BASH" ]; then echo $$BASH; \
	  else if [ -x /bin/bash ]; then echo /bin/bash; \
	  else echo sh; fi ; fi)

#	Only built-in objects are ever built: busybox has no loadable modules.

KBUILD_BUILTIN := 1

export KBUILD_BUILTIN
export KBUILD_SRC

# Beautify output
# ---------------------------------------------------------------------------
#
# Normally, we echo the whole command before executing it. By making
# that echo $($(quiet)$(cmd)), we now have the possibility to set
# $(quiet) to choose other forms of output instead, e.g.
#
#         quiet_cmd_cc_o_c = Compiling $(RELDIR)/$@
#         cmd_cc_o_c       = $(CC) $(c_flags) -c -o $@ $<
#
# If $(quiet) is empty, the whole command will be printed.
# If it is set to "quiet_", only the short version will be printed.
# If it is set to "silent_", nothing wil be printed at all, since
# the variable $(silent_cmd_cc_o_c) doesn't exist.
#
# A simple variant is to prefix commands with $(Q) - that's useful
# for commands that shall be hidden in non-verbose mode.
#
#	$(Q)ln $@ :<
#
# If KBUILD_VERBOSE equals 0 then the above command will be hidden.
# If KBUILD_VERBOSE equals 1 then the above command is displayed.

ifeq ($(KBUILD_VERBOSE),1)
  quiet =
  Q =
else
  quiet=quiet_
  Q = @
endif

# If the user is running make -s (silent mode), suppress echoing of
# commands
# make-4.0 (and later) keep single letter options in the 1st word of MAKEFLAGS.

ifeq ($(filter 3.%,$(MAKE_VERSION)),)
short-opts := $(firstword -$(MAKEFLAGS))
else
short-opts := $(filter-out --%,$(MAKEFLAGS))
endif

ifneq ($(findstring s,$(short-opts)),)
  quiet=silent_
endif

export quiet Q KBUILD_VERBOSE


# Look for make include files relative to root of kernel src
MAKEFLAGS += --include-dir=$(srctree)

HOSTCC  	= $(HOST_COMPILER)
HOSTCXX  	= g++
HOSTCFLAGS	:=
HOSTCXXFLAGS	:=
# We need some generic definitions
include $(srctree)/scripts/Kbuild.include

HOSTCFLAGS	+= $(call hostcc-option,-Wall -Wstrict-prototypes -O2 -fomit-frame-pointer,)
HOSTCXXFLAGS	+= -O2

# For maximum performance (+ possibly random breakage, uncomment
# the following)

MAKEFLAGS += -rR

# Make variables (CC, etc...)

AS		= $(CROSS_COMPILE)as
CC		= $(CROSS_COMPILE)$(CROSS_COMPILER)
LD		= $(CC) -nostdlib
CPP		= $(CC) -E
AR		= $(CROSS_COMPILE)ar
NM		= $(CROSS_COMPILE)nm
STRIP		= $(CROSS_COMPILE)strip
OBJCOPY		= $(CROSS_COMPILE)objcopy
OBJDUMP		= $(CROSS_COMPILE)objdump
WINDRES		= $(CROSS_COMPILE)windres
PKG_CONFIG	?= $(CROSS_COMPILE)pkg-config
AWK		= awk
PERL		= perl

# Handle MSYS2 weirdness
ifneq ($(CROSS_COMPILE),)
ifeq ($(shell _= command -v $(AR)),)
AR := $(CROSS_COMPILE)gcc-ar
STRIP := strip
WINDRES := windres
endif
endif

EXEEXT		=


CFLAGS		:= $(CFLAGS)
# Added only to final link stage of busybox binary
CFLAGS_busybox	:= $(CFLAGS_busybox)
CPPFLAGS	:= $(CPPFLAGS)
AFLAGS		:= $(AFLAGS)
LDFLAGS		:= $(LDFLAGS)
LDLIBS		:=

CONFIG_PLATFORM_MINGW32 ?=
ifeq ($(CONFIG_PLATFORM_MINGW32),)
CONFIG_PLATFORM_MINGW32 := $(shell grep ^CONFIG_PLATFORM_MINGW32= .config 2>/dev/null)
CONFIG_PLATFORM_MINGW32 := $(subst CONFIG_PLATFORM_MINGW32=,,$(CONFIG_PLATFORM_MINGW32))
CONFIG_PLATFORM_MINGW32 := $(subst ",,$(CONFIG_PLATFORM_MINGW32))
#")
endif

# Try various methods to get a more specific EXTRAVERSION, but only
# if EXTRAVERSION is default '.git'
ifeq ($(EXTRAVERSION),.git)
# Ask git: tag names are of the form "v<version>", e.g. "v1.38.1".
# git describe gives "v1.38.1" on the tag itself and
# "v1.38.1-12-gabc1234" after it.
extraversion := $(shell cd $(srctree) && git describe --match 'v[0-9]*' 2>/dev/null)
ifeq ($(strip $(extraversion)),)
# That didn't work, look for a .release_ver file
extraversion := $(shell grep '^v' $(srctree)/.release_ver 2>/dev/null)
ifeq ($(strip $(extraversion)),)
# That didn't work either, look at name of source directory
e1 := $(shell basename $(srctree) | grep '^busybox-w32-v')
ifneq ($(strip $(e1)),)
extraversion := $(subst busybox-w32-,,$(e1))
endif
endif
endif

ifneq ($(strip $(extraversion)),)
# Strip the "v<version>" tag prefix.  What remains is "-N-gHASH"
# (commits since the tag) or nothing (we are on the tag itself).
extraversion := $(subst v$(VERSION).$(PATCHLEVEL).$(SUBLEVEL),,$(extraversion))
ifneq ($(strip $(extraversion)),)
EXTRAVERSION := .git$(extraversion)
else
EXTRAVERSION :=
endif
endif
endif

# Read KERNELRELEASE from .kernelrelease (if it exists)
KERNELRELEASE = $(shell cat .kernelrelease 2> /dev/null)
KERNELVERSION = $(VERSION).$(PATCHLEVEL).$(SUBLEVEL)$(EXTRAVERSION)

export	VERSION PATCHLEVEL SUBLEVEL KERNELRELEASE KERNELVERSION \
	ARCH CONFIG_SHELL HOSTCC HOSTCFLAGS CROSS_COMPILE AS LD CC \
	CPP AR NM STRIP OBJCOPY OBJDUMP WINDRES MAKE AWK PERL \
	HOSTCXX HOSTCXXFLAGS

export CPPFLAGS LDFLAGS
export CFLAGS AFLAGS

# Files to ignore in find ... statements

RCS_FIND_IGNORE := \( -name SCCS -o -name BitKeeper -o -name .svn -o -name CVS -o -name .pc -o -name .hg -o -name .git \) -prune -o
export RCS_TAR_IGNORE := --exclude SCCS --exclude BitKeeper --exclude .svn --exclude CVS --exclude .pc --exclude .hg --exclude .git

# ===========================================================================
# Rules shared between *config targets and build targets

# Basic helpers built in scripts/
PHONY += scripts_basic
scripts_basic:
	$(Q)$(MAKE) $(build)=scripts/basic

# To avoid any implicit rule to kick in, define an empty command.
scripts/basic/%: scripts_basic ;

# This target generates Kbuild's and Config.in's from *.c files
PHONY += gen_build_files
gen_build_files: $(wildcard $(srctree)/*/*.c) $(wildcard $(srctree)/*/*/*.c) $(wildcard $(srctree)/embed/*)
	$(Q)$(srctree)/scripts/gen_build_files.sh $(srctree) $(objtree)

# bbox: we have helpers in applets/
# we depend on scripts_basic, since scripts/basic/fixdep
# must be built before any other host prog
PHONY += applets_dir
applets_dir: scripts_basic gen_build_files include/config/MARKER
	$(Q)$(MAKE) $(build)=applets

applets/%: applets_dir ;

PHONY += outputmakefile
# outputmakefile generates a Makefile in the output directory, if using a
# separate output directory. This allows convenient use of make in the
# output directory.
outputmakefile:
ifneq ($(KBUILD_SRC),)
	$(Q)$(CONFIG_SHELL) $(srctree)/scripts/mkmakefile \
	    $(srctree) $(objtree) $(VERSION) $(PATCHLEVEL)
endif

# To make sure we do not include .config for any of the *config targets
# catch them early, and hand them over to scripts/kconfig/Makefile
# It is allowed to specify more targets when calling make, including
# mixing *config targets and build targets.
# For example 'make oldconfig all'.
# Detect when mixed targets is specified, and make a second invocation
# of make so .config is not included in this case either (for *config).

no-dot-config-targets := clean mrproper distclean \
			 cscope TAGS tags help
#bbox# check% is removed from above

config-targets := 0
mixed-targets  := 0
dot-config     := 1

ifneq ($(filter $(no-dot-config-targets), $(MAKECMDGOALS)),)
	ifeq ($(filter-out $(no-dot-config-targets), $(MAKECMDGOALS)),)
		dot-config := 0
	endif
endif

ifneq ($(filter config %config,$(MAKECMDGOALS)),)
        config-targets := 1
        ifneq ($(filter-out config %config,$(MAKECMDGOALS)),)
                mixed-targets := 1
        endif
endif

ifeq ($(mixed-targets),1)
# ===========================================================================
# We're called with mixed targets (*config and build targets).
# Handle them one by one.

%:: FORCE
	$(Q)$(MAKE) -C $(srctree) KBUILD_SRC= $@

else
ifeq ($(config-targets),1)
# ===========================================================================
# *config targets only - make sure prerequisites are updated, and descend
# in scripts/kconfig to make the *config target

# Read arch specific Makefile to set KBUILD_DEFCONFIG as needed.
# KBUILD_DEFCONFIG may point out an alternative default configuration
# used for 'make defconfig'
-include $(srctree)/arch/$(ARCH)/Makefile
export KBUILD_DEFCONFIG

config: scripts_basic outputmakefile gen_build_files FORCE
	$(Q)mkdir -p include
	$(Q)$(MAKE) $(build)=scripts/kconfig $@
	$(Q)$(MAKE) -C $(srctree) KBUILD_SRC= .kernelrelease

%config: scripts_basic outputmakefile gen_build_files FORCE
	$(Q)mkdir -p include
	$(Q)$(MAKE) $(build)=scripts/kconfig $@
	$(Q)$(MAKE) -C $(srctree) KBUILD_SRC= .kernelrelease

else
# ===========================================================================
# Build targets only - this includes busybox, arch specific targets, clean
# targets and others. In general all targets except *config targets.

# Additional helpers built in scripts/
# Carefully list dependencies so we do not try to build scripts twice
# in parallel
PHONY += scripts
scripts: gen_build_files scripts_basic include/config/MARKER
	$(Q)$(MAKE) $(build)=$(@)

scripts_basic: include/autoconf.h

# Objects we will link into busybox / subdirs we need to visit
core-y		:= \
		applets/ \
		win32/resources/ \

libs-y		:= \
		archival/ \
		archival/libarchive/ \
		console-tools/ \
		coreutils/ \
		coreutils/libcoreutils/ \
		debianutils/ \
		e2fsprogs/ \
		editors/ \
		findutils/ \
		libbb/ \
		libbb/yescrypt/ \
		libpwdgrp/ \
		loginutils/ \
		miscutils/ \
		networking/ \
		procps/ \
		shell/ \
		util-linux/ \
		win32/ \

ifeq ($(dot-config),1)
# In this section, we need .config

# Read in dependencies to all Kconfig* files, make sure to run
# oldconfig if changes are detected.
-include .kconfig.d

-include .config

# If .config needs to be updated, it will be done via the dependency
# that autoconf has on .config.
# To avoid any implicit rule to kick in, define an empty command
.config .kconfig.d: ;

-include $(srctree)/arch/$(ARCH)/Makefile

# Now we can define CFLAGS etc according to .config
include $(srctree)/Makefile.flags

# If .config is newer than include/autoconf.h, someone tinkered
# with it and forgot to run make oldconfig.
# If kconfig.d is missing then we are probarly in a cleaned tree so
# we execute the config step to be sure to catch updated Kconfig files
include/autoconf.h: .kconfig.d .config $(wildcard $(srctree)/*/*.c) $(wildcard $(srctree)/*/*/*.c) | gen_build_files
	$(Q)$(MAKE) -f $(srctree)/Makefile silentoldconfig

include/usage.h: gen_build_files

else
# Dummy target needed, because used as prerequisite
include/autoconf.h: ;
endif

# The all: target is the default when no target is given on the
# command line.
# This allows a user to issue only 'make' to build busybox and the docs
all: busybox$(EXEEXT) doc

# arch Makefile may override CC so keep this after arch Makefile is included

busybox-dirs	:= $(patsubst %/,%,$(filter %/, $(core-y) $(core-m) $(libs-y) $(libs-m)))

busybox-alldirs	:= $(sort $(busybox-dirs) $(patsubst %/,%,$(filter %/, \
		     $(core-n) $(core-) $(libs-n) $(libs-) \
		)))

core-y		:= $(patsubst %/, %/built-in.o, $(core-y))
libs-y1		:= $(patsubst %/, %/lib.a, $(libs-y))
libs-y2		:= $(patsubst %/, %/built-in.o, $(libs-y))
libs-y		:= $(libs-y1) $(libs-y2)

# Build busybox
# ---------------------------------------------------------------------------
# busybox is linked from the built-in.o files of the directories listed in
# $(core-y) and the lib.a / built-in.o files of the directories in $(libs-y).

busybox-all  := $(core-y) $(libs-y)

# Rule to link busybox
# May be overridden by arch/$(ARCH)/Makefile
quiet_cmd_busybox__ ?= LINK    $@
      cmd_busybox__ ?= $(srctree)/scripts/trylink \
      "$@" \
      "$(CC)" \
      "$(CFLAGS) $(CFLAGS_busybox)" \
      "$(LDFLAGS) $(EXTRA_LDFLAGS)" \
      "$(core-y)" \
      "$(libs-y)" \
      "$(LDLIBS)" \
      $(CONFIG_EXTRA_LDLIBS) \
      && $(srctree)/scripts/generate_BUFSIZ.sh --post include/common_bufsiz.h
# ^^^ note: CONFIG_xyz strings already have double quotes: their value
# is '"LIB LIB2"', therefore $(CONFIG_EXTRA_LDLIBS) above must NOT be written
# as "$(CONFIG_EXTRA_LDLIBS)", it would be passed as ""LIB LIB2"",
# and LIB2 would end up in $9, not $8 (and lost or misinterpreted).

# Link of busybox
# First command is ':' to allow us to use + in front of the rule
define rule_busybox__
	:
	$(call cmd,busybox__)
	$(Q)echo 'cmd_$@ := $(cmd_busybox__)' > $(@D)/.$(@F).cmd
endef

# busybox image
busybox_unstripped$(EXEEXT): $(busybox-all) FORCE
	$(call if_changed_rule,busybox__)

busybox$(EXEEXT): busybox_unstripped$(EXEEXT)
ifeq ($(SKIP_STRIP),y)
	$(Q)cp $< $@
else
	$(Q)$(STRIP) -s --remove-section=.note --remove-section=.comment \
		busybox_unstripped$(EXEEXT) -o $@
# strip is confused by PIE executable and does not set exec bits
	$(Q)chmod a+x $@
endif

# The actual objects are generated when descending,
# make sure no implicit rule kicks in
$(sort $(busybox-all)): $(busybox-dirs) ;

# Handle descending into subdirectories listed in $(busybox-dirs)
# Preset locale variables to speed up the build process. Limit locale
# tweaks to this spot to avoid wrong language settings when running
# make menuconfig etc.
# Error messages still appears in the original language

PHONY += $(busybox-dirs)
$(busybox-dirs): prepare scripts
	$(Q)$(MAKE) $(build)=$@

# Build the kernel release string
# The KERNELRELEASE is stored in a file named .kernelrelease
# to be used when executing for example make install
#
# Take the contents of any files called localversion* and the config
# variable CONFIG_LOCALVERSION and append them to KERNELRELEASE.
# LOCALVERSION from the command line override all of this

nullstring :=
space      := $(nullstring) # end of line

___localver = $(objtree)/localversion* $(srctree)/localversion*
__localver  = $(sort $(wildcard $(___localver)))
# skip backup files (containing '~')
_localver = $(foreach f, $(__localver), $(if $(findstring ~, $(f)),,$(f)))

localver = $(subst $(space),, \
	   $(shell cat /dev/null $(_localver)) \
	   $(patsubst "%",%,$(CONFIG_LOCALVERSION)))

# Store (new) KERNELRELEASE string in .kernelrelease
kernelrelease = $(KERNELVERSION)$(localver)
.kernelrelease: FORCE
	$(Q)rm -f $@
	$(Q)echo $(kernelrelease) > $@


# Things we need to do before we recursively start building are listed
# in "prepare".
# A multi level approach is used. prepareN is processed before prepareN-1.

# Listed in dependency order
PHONY += prepare archprepare prepare0 prepare1 prepare2 prepare3

# prepare3 checks, when building in a separate output directory, that
# make has not been executed in the source tree $(srctree)
prepare3: .kernelrelease
ifneq ($(KBUILD_SRC),)
	@echo '  Using $(srctree) as source for busybox'
	$(Q)if [ -f $(srctree)/.config ]; then \
		echo "  $(srctree) is not clean, please run 'make mrproper'";\
		echo "  in the '$(srctree)' directory.";\
		/bin/false; \
	fi;
endif

# prepare2 creates a makefile if using a separate output directory
prepare2: prepare3 outputmakefile

prepare1: prepare2 include/config/MARKER include/BB_VER.h

archprepare: prepare1 scripts_basic applets_dir

prepare0: archprepare FORCE
	$(Q)$(MAKE) $(build)=.

# All the preparing..
prepare: prepare0

# 	Split autoconf.h into include/linux/config/*
quiet_cmd_gen_bbconfigopts = GEN     include/bbconfigopts.h
      cmd_gen_bbconfigopts = $(srctree)/scripts/mkconfigs include/bbconfigopts.h include/bbconfigopts_bz2.h
quiet_cmd_gen_common_bufsiz = GEN     include/common_bufsiz.h
      cmd_gen_common_bufsiz = $(srctree)/scripts/generate_BUFSIZ.sh include/common_bufsiz.h
quiet_cmd_split_autoconf   = SPLIT   include/autoconf.h -> include/config/*
      cmd_split_autoconf   = scripts/basic/split-include include/autoconf.h include/config
quiet_cmd_gen_embedded_scripts = GEN     include/embedded_scripts.h
      cmd_gen_embedded_scripts = $(srctree)/scripts/embedded_scripts include/embedded_scripts.h $(srctree)/embed $(srctree)/applets_sh
#bbox# piggybacked generation of few .h files
include/config/MARKER: scripts/basic/split-include include/autoconf.h $(wildcard $(srctree)/embed/*) $(wildcard $(srctree)/applets_sh/*) $(srctree)/scripts/embedded_scripts
	$(call cmd,split_autoconf)
	$(call cmd,gen_bbconfigopts)
	$(call cmd,gen_common_bufsiz)
	$(call cmd,gen_embedded_scripts)
	@touch $@

# Generate some files
# ---------------------------------------------------------------------------

define filechk_BB_VER.h
	(echo \#define BB_VER \"$(KERNELRELEASE)\";)
endef

include/BB_VER.h: $(srctree)/Makefile .config .kernelrelease FORCE
	$(call filechk,BB_VER.h)

###
# Cleaning is done on three levels.
# make clean     Delete most generated files
#                Leave enough to build external modules
# make mrproper  Delete the current configuration, and all generated files
# make distclean Remove editor backup files, patch leftover files and the like

# Directories & files removed with 'make clean'
CLEAN_DIRS  += _install 0_lib
CLEAN_FILES +=	busybox$(EXEEXT) busybox_unstripped* busybox.links \
		busybox*.suid busybox*.nosuid \
                .kernelrelease

# Directories & files removed with 'make mrproper'
MRPROPER_DIRS  += include/config
MRPROPER_FILES += .config .config.old \
		  include/NUM_APPLETS.h \
		  include/BB_VER.h \
		  include/common_bufsiz.h \
		  include/autoconf.h \
		  include/bbconfigopts.h \
		  include/bbconfigopts_bz2.h \
		  include/embedded_scripts.h \
		  include/usage_compressed.h \
		  include/applet_tables.h \
		  include/applets.h \
		  include/usage.h \
		  applets/usage \
		  win32/resources/busybox-w32.manifest \
		  .kernelrelease Module.symvers tags TAGS cscope* \
		  busybox_old

# clean - Delete most, but leave enough to build external modules
#
clean: rm-dirs  := $(CLEAN_DIRS)
clean: rm-files := $(CLEAN_FILES)
clean-dirs      := $(addprefix _clean_,$(srctree) $(busybox-alldirs))

PHONY += $(clean-dirs) clean archclean
$(clean-dirs):
	$(Q)$(MAKE) $(clean)=$(patsubst _clean_%,%,$@)

clean: archclean $(clean-dirs)
	$(call cmd,rmdirs)
	$(call cmd,rmfiles)
	@find . $(RCS_FIND_IGNORE) \
		\( -name '*.[oas]' -o -name '*.ko' -o -name '.*.cmd' \
		-o -name '.*.d' -o -name '.*.tmp' -o -name '*.mod.c' \) \
		-type f -print | xargs rm -f

PHONY += doc-clean
doc-clean: rm-files := docs/busybox.pod \
		  docs/BusyBox.html docs/busybox.1 docs/BusyBox.txt
doc-clean:
	$(call cmd,rmfiles)

# mrproper - Delete all generated files, including .config
#
mrproper: rm-dirs  := $(wildcard $(MRPROPER_DIRS))
mrproper: rm-files := $(wildcard $(MRPROPER_FILES))
mrproper-dirs      := $(addprefix _mrproper_,scripts)

PHONY += $(mrproper-dirs) mrproper archmrproper
$(mrproper-dirs):
	$(Q)$(MAKE) $(clean)=$(patsubst _mrproper_%,%,$@)

mrproper: clean archmrproper $(mrproper-dirs)
	$(call cmd,rmdirs)
	$(call cmd,rmfiles)
	@find . -name Config.src | sed 's/.src$$/.in/' | xargs -r rm -f
	@find . -name Kbuild.src | sed 's/.src$$//' | xargs -r rm -f

# distclean
#
PHONY += distclean

distclean: mrproper
	@find $(srctree) $(RCS_FIND_IGNORE) \
		\( -name '*.orig' -o -name '*.rej' -o -name '*~' \
		-o -name '*.bak' -o -name '#*#' -o -name '.*.orig' \
		-o -name '.*.rej' -o -name '*.tmp' -o -size 0 \
		-o -name '*%' -o -name '.*.cmd' -o -name 'core' \) \
		-type f -print | xargs rm -f

# Brief documentation of the typical targets used
# ---------------------------------------------------------------------------

boards := $(wildcard $(srctree)/configs/*_defconfig)
boards := $(notdir $(boards))

-include $(srctree)/Makefile.help

# Generate tags for editors
# ---------------------------------------------------------------------------

define all-sources
	( find -regex '.*\.[ch]$$' )
endef

quiet_cmd_cscope-file = FILELST cscope.files
      cmd_cscope-file = (echo \-k; echo \-q; $(all-sources)) > cscope.files

quiet_cmd_cscope = MAKE    cscope.out
      cmd_cscope = cscope -b

cscope: FORCE
	$(call cmd,cscope-file)
	$(call cmd,cscope)

quiet_cmd_TAGS = MAKE   $@
define cmd_TAGS
	rm -f $@; \
	ETAGSF=`etags --version | grep -i exuberant >/dev/null &&     \
                echo "-I __initdata,__exitdata,__acquires,__releases  \
                      -I EXPORT_SYMBOL,EXPORT_SYMBOL_GPL              \
                      --extra=+f --c-kinds=+px"`;                     \
                $(all-sources) | xargs etags $$ETAGSF -a
endef

TAGS: FORCE
	$(call cmd,TAGS)


quiet_cmd_tags = MAKE   $@
define cmd_tags
	rm -f $@; \
	CTAGSF=`ctags --version | grep -i exuberant >/dev/null &&     \
                echo "-I __initdata,__exitdata,__acquires,__releases  \
                      -I EXPORT_SYMBOL,EXPORT_SYMBOL_GPL              \
                      --extra=+f --c-kinds=+px"`;                     \
                $(all-sources) | xargs ctags $$CTAGSF -a
endef

tags: FORCE
	$(call cmd,tags)


endif #ifeq ($(config-targets),1)
endif #ifeq ($(mixed-targets),1)

PHONY += checkstack
checkstack:
	$(OBJDUMP) -d busybox$(EXEEXT) | $(PERL) $(src)/scripts/checkstack.pl $(ARCH)

kernelrelease:
	$(if $(wildcard .kernelrelease), $(Q)echo $(KERNELRELEASE), \
	$(error kernelrelease not valid - run 'make *config' to update it))
kernelversion:
	@echo $(KERNELVERSION)

# Single targets
# ---------------------------------------------------------------------------
# Single targets are compatible with:
# - build whith mixed source and output
# - build with separate output dir 'make O=...'
#
#  target-dir => where to store outputfile
#  build-dir  => directory in source tree to use

build-dir  = $(patsubst %/,%,$(dir $@))
target-dir = $(dir $@)

%.s: %.c prepare scripts FORCE
	$(Q)$(MAKE) $(build)=$(build-dir) $(target-dir)$(notdir $@)
%.i: %.c prepare scripts FORCE
	$(Q)$(MAKE) $(build)=$(build-dir) $(target-dir)$(notdir $@)
%.o: %.c prepare scripts FORCE
	$(Q)$(MAKE) $(build)=$(build-dir) $(target-dir)$(notdir $@)
%.s: %.S prepare scripts FORCE
	$(Q)$(MAKE) $(build)=$(build-dir) $(target-dir)$(notdir $@)
%.o: %.S prepare scripts FORCE
	$(Q)$(MAKE) $(build)=$(build-dir) $(target-dir)$(notdir $@)

%/: prepare scripts FORCE
	$(Q)$(MAKE) $(build)=$(build-dir)
/: prepare scripts FORCE
	$(Q)$(MAKE) $(build)=$(build-dir)

# FIXME Should go into a make.lib or something
# ===========================================================================

quiet_cmd_rmdirs = $(if $(wildcard $(rm-dirs)),CLEAN   $(wildcard $(rm-dirs)))
      cmd_rmdirs = rm -rf $(rm-dirs)

quiet_cmd_rmfiles = $(if $(wildcard $(rm-files)),CLEAN   $(wildcard $(rm-files)))
      cmd_rmfiles = rm -f $(rm-files)

# read all saved command lines

targets := $(wildcard $(sort $(targets)))
cmd_files := $(wildcard .*.cmd $(foreach f,$(targets),$(dir $(f)).$(notdir $(f)).cmd))

ifneq ($(cmd_files),)
  $(cmd_files): ;	# Do not try to update included dependency files
  include $(cmd_files)
endif

# Shorthand for $(Q)$(MAKE) -f scripts/Makefile.clean obj=dir
# Usage:
# $(Q)$(MAKE) $(clean)=dir
clean := -f $(if $(KBUILD_SRC),$(srctree)/)scripts/Makefile.clean obj

endif	# skip-makefile

PHONY += FORCE
FORCE:

-include $(srctree)/Makefile.custom

# Declare the contents of the .PHONY variable as phony.  We keep that
# information in a variable se we can use it in if_changed and friends.
.PHONY: $(PHONY)
