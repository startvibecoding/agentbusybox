# ===========================================================================
# Kernel configuration targets
# These targets are used from top-level makefile

# If w64devkit isn't run via w64devkit.exe W64DEVKIT won't be
# defined.  If HOSTCC targets the mingw32 platform set W64DEVKIT.
ifndef W64DEVKIT
	host_target := $(shell $(HOSTCC) -v 2>&1 | grep ^Target:)
	host_platform := $(lastword $(subst -, ,$(host_target)))
	ifeq ($(host_platform),mingw32)
		W64DEVKIT := 1
		export W64DEVKIT
	endif
endif

PHONY += oldconfig menuconfig config silentoldconfig

menuconfig: $(obj)/mconf
ifdef W64DEVKIT
	$(Q)$(MAKE) $(build)=scripts/kconfig/libcurses
endif
	$(Q)$(MAKE) $(build)=scripts/kconfig/lxdialog
	$< Config.in

config: $(obj)/conf
	$< Config.in

# Mtime granularity problem.
# It was observed that these commands:
# make allnoconfig; sed -i -e '/CONFIG_TRUE/s/.*/CONFIG_TRUE=y/' .config; make
# sometimes produce busybox with "true" applet still disabled.
# This is caused by .config updated by sed having mtime which is still
# equal to (not bigger than) include/autoconf.h's mtime,
# and thus 2nd make does not regenerate include/autoconf.h.
# Waiting for 1 second after non-interactive "make XXXXconfig"
# prevents this from happening.
#
# We'd like to detect whether filesystem we are on has coarse mtimes,
# but can't do it yet, bbox ls hasn't got --full-time.
#MTIME_IS_COARSE:=@ls --full-time -ld | grep -F .000 >/dev/null
MTIME_IS_COARSE:=@true

oldconfig: $(obj)/conf
	$< -o Config.in
	$(MTIME_IS_COARSE) && sleep 1

silentoldconfig: $(obj)/conf
	$< -s Config.in
	$(MTIME_IS_COARSE) && sleep 1

PHONY += randconfig allyesconfig allnoconfig defconfig

randconfig: $(obj)/conf
	$< -r Config.in
	$(MTIME_IS_COARSE) && sleep 1

allyesconfig: $(obj)/conf
	$< -y Config.in
	$(MTIME_IS_COARSE) && sleep 1

allnoconfig: $(obj)/conf
	$< -n Config.in
	$(MTIME_IS_COARSE) && sleep 1

defconfig: $(obj)/conf
ifeq ($(KBUILD_DEFCONFIG),)
	$< -d Config.in
else
	@echo *** Default configuration is based on '$(KBUILD_DEFCONFIG)'
	$(Q)$< -D $(KBUILD_DEFCONFIG) Config.in
endif
	$(MTIME_IS_COARSE) && sleep 1

%_defconfig: $(obj)/conf
	$(Q)$< -D configs/$@ Config.in
	$(MTIME_IS_COARSE) && sleep 1

# Help text used by make help
help:
	@echo  '  config	  - Update current config utilising a line-oriented program'
	@echo  '  menuconfig	  - Update current config utilising a menu based program'
	@echo  '  oldconfig	  - Update current config utilising a provided .config as base'
	@echo  '  randconfig	  - New config with random answer to all options'
	@echo  '  defconfig	  - New config with default answer to all options'
	@echo  '  allyesconfig	  - New config where all options are accepted with yes'
	@echo  '  allnoconfig	  - New config where all options are answered with no'

# ===========================================================================
# Shared Makefile for the various kconfig executables:
# conf:	  Used for defconfig, oldconfig and related targets
# mconf:  Used for the menuconfig target.
#         Utilizes the lxdialog package
# object files used by all kconfig flavours

hostprogs-y	:= conf mconf
conf-objs	:= conf.o  zconf.tab.o
mconf-objs	:= mconf.o zconf.tab.o

clean-files	:= zconf.tab.c lex.zconf.c zconf.hash.c
subdir- += lxdialog

# Add environment specific flags
HOST_EXTRACFLAGS += $(shell $(CONFIG_SHELL) $(srctree)/$(src)/check.sh $(HOSTCC) $(HOSTCFLAGS))

# generated files seem to need this to find local include files
HOSTCFLAGS_lex.zconf.o	:= -I$(src)
HOSTCFLAGS_zconf.tab.o	:= -I$(src)

$(obj)/zconf.tab.o: $(obj)/lex.zconf.c $(obj)/zconf.hash.c


###
# The following requires flex/bison/gperf
# By default we use the _shipped versions, uncomment the following line if
# you are modifying the flex/bison src.
# LKC_GENPARSER := 1

ifdef LKC_GENPARSER

$(obj)/zconf.tab.c: $(src)/zconf.y
$(obj)/lex.zconf.c: $(src)/zconf.l
$(obj)/zconf.hash.c: $(src)/zconf.gperf

%.tab.c: %.y
	bison -l -b $* -p $(notdir $*) $<
	cp $@ $@_shipped

lex.%.c: %.l
	flex -L -P$(notdir $*) -o$@ $<
	cp $@ $@_shipped

%.hash.c: %.gperf
	gperf < $< > $@
	cp $@ $@_shipped

endif
