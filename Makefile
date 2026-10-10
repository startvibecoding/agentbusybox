VERSION = 1
PATCHLEVEL = 38
SUBLEVEL = 1
EXTRAVERSION = .git

# Colon is used as a separator in makefiles.  Strip any drive prefix
# from the current directory to avoid confusion.
CURDIR := $(lastword $(subst :, ,$(CURDIR)))

# To see a list of typical targets execute "make help"
# More info can be located in ./README.md
# Comments in this file are targeted only to the developer, do not
# expect to learn how the kernel build system works reading this file.

# Do not print "Entering directory ..."
MAKEFLAGS += --no-print-directory

# To put more focus on warnings, be less verbose as default
# Use 'make V=1' to see the full commands
ifeq ("$(origin V)", "command line")
  KBUILD_VERBOSE = $(V)
endif
ifndef KBUILD_VERBOSE
  KBUILD_VERBOSE = 0
endif

# ===========================================================================
# Out-of-tree build support
#
# "make O=dir/to/store/output/files/" invokes a second make from the
# output directory; this same Makefile is then read again with KBUILD_SRC
# pointing at the source tree, so several build trees (e.g. build-linux/
# and build-win64/) can coexist.  The source tree must be free of
# generated files for this to work - run "make mrproper" if it isn't.
# ===========================================================================
ifeq ($(KBUILD_SRC),)

ifdef O
  ifeq ("$(origin O)", "command line")
    KBUILD_OUTPUT := $(O)
  endif
endif

# That's our default target when none is given on the command line
PHONY := _all
_all:

ifneq ($(KBUILD_OUTPUT),)
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
objtree		:= $(CURDIR)
src		:= $(srctree)
obj		:= $(objtree)

VPATH		:= $(srctree)

export srctree objtree VPATH TOPDIR

# Cross compiling and selecting different set of gcc/bin-utils
# -------------------------------------------------------------
# CROSS_COMPILE specifies the prefix used for target tools.  It is a build
# environment property, not a BusyBox feature: pass it on the command line
# or export it (for example, CROSS_COMPILE=x86_64-w64-mingw32-).  Keeping it
# out of .config makes profiles portable between build hosts and prevents a
# stale compiler path from being silently reused.
CROSS_COMPILE ?=

# ARCH selects the per-arch compiler tuning in arch/$(ARCH)/Makefile.
# The only tunings we ship are i386 and x86_64; when cross compiling,
# infer it from the CROSS_COMPILE prefix (e.g. i686-w64-mingw32- -> i386).
ifneq ($(CROSS_COMPILE),)
SUBARCH := $(shell echo $(CROSS_COMPILE) | cut -d- -f1 | sed 's:^.*/::g')
else
SUBARCH := $(shell uname -m)
endif
SUBARCH := $(shell echo $(SUBARCH) | sed -e s/i.86/i386/)

ARCH ?= $(SUBARCH)

# SHELL used by kbuild
CONFIG_SHELL := $(shell if [ -x "$$BASH" ]; then echo $$BASH; \
	  else if [ -x /bin/bash ]; then echo /bin/bash; \
	  else echo sh; fi ; fi)

# We always link everything into one binary (no modules).  This is what
# scripts/Makefile.build checks when descending into every directory:
KBUILD_BUILTIN := 1
# Never run the sparse checker over the sources (scripts/Makefile.build
# tests KBUILD_CHECKSRC and must see a defined value):
KBUILD_CHECKSRC := 0
export KBUILD_BUILTIN KBUILD_CHECKSRC KBUILD_SRC

# Beautify output
# ---------------------------------------------------------------------------
# Normally, we echo the whole command before executing it. By making
# that echo $($(quiet)cmd_$(1)), we now have the possibility to set
# $(quiet) to choose other forms of output instead, e.g.
#
#         quiet_cmd_cc_o_c = Compiling $(RELDIR)/$@
#         cmd_cc_o_c       = $(CC) $(c_flags) -c -o $@ $<
#
# If $(quiet) is empty, the whole command will be printed.
# If it is set to "quiet_", only the short version will be printed.
# If it is set to "silent_", nothing will be printed at all, since
# the variable $(silent_cmd_cc_o_c) doesn't exist.
#
# A simple variant is to prefix commands with $(Q) - that's useful
# for commands that shall be hidden in non-verbose mode.
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

# Look for make include files relative to root of src
MAKEFLAGS += --include-dir=$(srctree)

# Host helpers always run on the build machine. HOSTCC and CC remain normal
# make variables, so callers can override either one when needed.
HOSTCC  	?= gcc
HOSTCFLAGS	:=
# We need some generic definitions
include $(srctree)/scripts/Kbuild.include

HOSTCFLAGS	+= $(call hostcc-option,-Wall -Wstrict-prototypes -O2 -fomit-frame-pointer,)

# For maximum performance (+ possibly random breakage, uncomment
# the following)
MAKEFLAGS += -rR

# Make variables (CC, etc...)

AS	= $(CROSS_COMPILE)as
CC	= $(CROSS_COMPILE)gcc
LD	= $(CC) -nostdlib
CPP	= $(CC) -E
AR	= $(CROSS_COMPILE)ar
NM	= $(CROSS_COMPILE)nm
STRIP	= $(CROSS_COMPILE)strip
OBJDUMP	= $(CROSS_COMPILE)objdump
WINDRES	= $(CROSS_COMPILE)windres

# Handle MSYS2 weirdness
ifneq ($(CROSS_COMPILE),)
ifeq ($(shell _= command -v $(AR)),)
AR := $(CROSS_COMPILE)gcc-ar
STRIP := strip
WINDRES := windres
endif
endif

CFLAGS		:= $(CFLAGS)
# Added only to final link stage of busybox binary
CFLAGS_busybox	:= $(CFLAGS_busybox)
CPPFLAGS	:= $(CPPFLAGS)
AFLAGS		:= $(AFLAGS)
LDFLAGS		:= $(LDFLAGS)
LDLIBS		:=
EXEEXT		=

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

KERNELVERSION = $(VERSION).$(PATCHLEVEL).$(SUBLEVEL)$(EXTRAVERSION)
KERNELRELEASE := $(KERNELVERSION)

export	VERSION PATCHLEVEL SUBLEVEL KERNELRELEASE KERNELVERSION \
	ARCH CONFIG_SHELL HOSTCC HOSTCFLAGS CROSS_COMPILE AS LD CC \
	CPP AR NM STRIP OBJDUMP WINDRES MAKE
export CPPFLAGS LDFLAGS CFLAGS AFLAGS

# Files to ignore in find ... statements
RCS_FIND_IGNORE := \( -name SCCS -o -name BitKeeper -o -name .svn -o -name CVS -o -name .pc -o -name .hg -o -name .git \) -prune -o

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
gen_build_files: $(wildcard $(srctree)/*/*.c) $(wildcard $(srctree)/*/*/*.c)
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

# ===========================================================================
# To make sure we do not include .config for any of the *config targets
# catch them early, and hand them over to scripts/kconfig/Makefile

no-dot-config-targets := clean mrproper distclean help

config-targets := 0
dot-config     := 1

ifneq ($(filter $(no-dot-config-targets), $(MAKECMDGOALS)),)
	ifeq ($(filter-out $(no-dot-config-targets), $(MAKECMDGOALS)),)
		dot-config := 0
	endif
endif

ifneq ($(filter config %config,$(MAKECMDGOALS)),)
	config-targets := 1
endif

ifeq ($(config-targets),1)
# ===========================================================================
# *config targets only - make sure prerequisites are updated, and descend
# in scripts/kconfig to make the *config target

config: scripts_basic outputmakefile gen_build_files FORCE
	$(Q)mkdir -p include
	$(Q)$(MAKE) $(build)=scripts/kconfig $@

%config: scripts_basic outputmakefile gen_build_files FORCE
	$(Q)mkdir -p include
	$(Q)$(MAKE) $(build)=scripts/kconfig $@

else
# ===========================================================================
# Build targets only - this includes busybox, arch specific targets
# and clean targets.

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

# Architecture specific compiler tuning (if any)
-include $(srctree)/arch/$(ARCH)/Makefile

# Now we can define CFLAGS etc according to .config
include $(srctree)/Makefile.flags

# If .config is newer than include/autoconf.h, someone tinkered
# with it and forgot to run make oldconfig.
# If kconfig.d is missing then we are probably in a cleaned tree so
# we execute the config step to be sure to catch updated Kconfig files
include/autoconf.h: .kconfig.d .config $(wildcard $(srctree)/*/*.c) $(wildcard $(srctree)/*/*/*.c) | gen_build_files
	$(Q)$(MAKE) -f $(srctree)/Makefile silentoldconfig

include/usage.h: gen_build_files

else
# Dummy target needed, because used as prerequisite
include/autoconf.h: ;
endif

# The all: target is the default when no target is given on the
# command line.  Generate the documentation with a separate "make doc".
all: busybox$(EXEEXT)

busybox-dirs	:= $(patsubst %/,%,$(filter %/, $(core-y) $(libs-y)))

core-y		:= $(patsubst %/, %/built-in.o, $(core-y))
libs-y1		:= $(patsubst %/, %/lib.a, $(libs-y))
libs-y2		:= $(patsubst %/, %/built-in.o, $(libs-y))
libs-y		:= $(libs-y1) $(libs-y2)

# Build busybox
# ---------------------------------------------------------------------------
# busybox is built from the objects in the directories listed in core-y
# and libs-y above: each directory contributes a lib.a and a built-in.o.
# Ordering when linking is important, and applets/ must be first.
#
# busybox
#   ^
#   |
#   +-< applets/applets.o, win32/resources/... (core-y)
#   |
#   +--< $(libs-y): all the applet and library directories
#
busybox-all  := $(core-y) $(libs-y)

# Rule to link busybox
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
      && NM="$(NM)" CC="$(CC)" $(srctree)/scripts/generate_BUFSIZ.sh --post include/common_bufsiz.h
# ^^^ note: CONFIG_xyz strings already have double quotes: their value
# is '"LIB LIB2"', therefore $(CONFIG_EXTRA_LDLIBS) above must NOT be written
# as "$(CONFIG_EXTRA_LDLIBS)", it would be passed as ""LIB LIB2"",
# and LIB2 would end up in $9, not $8 (and lost or misinterpreted).

# busybox image
busybox_unstripped$(EXEEXT): $(busybox-all) FORCE
	$(call if_changed,busybox__)

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

PHONY += $(busybox-dirs)
$(busybox-dirs): prepare scripts
	$(Q)$(MAKE) $(build)=$@

# Things we need to do before we recursively start building
# are listed in "prepare".
# A multi level approach is used. prepareN is processed before prepareN-1.

# Listed in dependency order
PHONY += prepare archprepare prepare0 prepare1 prepare2 prepare3

# prepare3 checks that we do not mix in-tree and out-of-tree builds:
# if $(srctree) contains generated files, O= builds would silently
# pick them up via VPATH - refuse instead.
prepare3:
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

# Generate include/config/*.h, include/bbconfigopts.h,
# include/common_bufsiz.h and include/embedded_scripts.h
quiet_cmd_gen_bbconfigopts = GEN     include/bbconfigopts.h
      cmd_gen_bbconfigopts = $(srctree)/scripts/mkconfigs include/bbconfigopts.h include/bbconfigopts_bz2.h
quiet_cmd_gen_common_bufsiz = GEN     include/common_bufsiz.h
      cmd_gen_common_bufsiz = NM="$(NM)" CC="$(CC)" $(srctree)/scripts/generate_BUFSIZ.sh include/common_bufsiz.h
quiet_cmd_split_autoconf   = SPLIT   include/autoconf.h -> include/config/*
      cmd_split_autoconf   = scripts/basic/split-include include/autoconf.h include/config
quiet_cmd_gen_embedded_scripts = GEN     include/embedded_scripts.h
      cmd_gen_embedded_scripts = $(srctree)/scripts/embedded_scripts include/embedded_scripts.h $(srctree)/embed $(srctree)/applets_sh
#bbox: piggybacked generation of few .h files
include/config/MARKER: scripts/basic/split-include include/autoconf.h $(wildcard $(srctree)/embed/*) $(wildcard $(srctree)/applets_sh/*) $(srctree)/scripts/embedded_scripts
	$(call cmd,split_autoconf)
	$(call cmd,gen_bbconfigopts)
	$(call cmd,gen_common_bufsiz)
	$(call cmd,gen_embedded_scripts)
	@touch $@

# The busybox version string lives in include/BB_VER.h.
# KERNELVERSION can change from a few different places (git state,
# Makefile edits), so this check is forced on all builds; filechk
# avoids touching the file if the generated content is unchanged.
define filechk_BB_VER.h
	(echo \#define BB_VER \"$(KERNELRELEASE)\";)
endef

include/BB_VER.h: $(srctree)/Makefile .config FORCE
	$(call filechk,BB_VER.h)

# ---------------------------------------------------------------------------
# Cleaning is done on three levels.
# make clean     Delete most generated files
# make mrproper  Delete the current configuration, and all generated files
# make distclean Remove editor backup files, patch leftover files and the like

# Directories & files removed with 'make clean'
CLEAN_DIRS  += _install 0_lib
CLEAN_FILES +=	busybox$(EXEEXT) busybox.exe busybox_unstripped* busybox.links \
		busybox*.suid busybox*.nosuid \
		.tmp_version

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
		  .tmp_version tags TAGS cscope* \
		  busybox_old

# clean - Delete most generated files
clean: rm-dirs  := $(CLEAN_DIRS)
clean: rm-files := $(CLEAN_FILES)
clean-dirs      := $(addprefix _clean_,$(srctree) $(busybox-dirs))

PHONY += $(clean-dirs) clean
$(clean-dirs):
	$(Q)$(MAKE) $(clean)=$(patsubst _clean_%,%,$@)

clean: $(clean-dirs)
	$(call cmd,rmdirs)
	$(call cmd,rmfiles)
	@find . $(RCS_FIND_IGNORE) \
		\( -name '*.[oas]' -o -name '.*.cmd' \
		-o -name '.*.d' -o -name '.*.tmp' \) \
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

PHONY += $(mrproper-dirs) mrproper
$(mrproper-dirs):
	$(Q)$(MAKE) $(clean)=$(patsubst _mrproper_%,%,$@)

mrproper: clean $(mrproper-dirs)
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

endif #ifeq ($(config-targets),1)

# Single targets
# ---------------------------------------------------------------------------
# Handy for testing compilation of one file: "make coreutils/ls.o"
# Works with out-of-tree builds too.

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

# Declare the contents of the .PHONY variable as phony. We keep that
# information in a variable so we can use it in if_changed and friends.
.PHONY: $(PHONY)
