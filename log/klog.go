/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Austin Zhai
 * All rights reserved.
 */
package log

import "k8s.io/klog/v2"

type KLog struct {
	verbosities []int // mapping for Level and klog verbosities
}

// just a wrapper for klog in kubernetes
// default verbosities see: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md
func NewKLog() *KLog {
	return &KLog{
		verbosities: []int{
			0,
			5, // LevelTrace
			4, // LevelDebug
			3, // LevelInfo
			2, // LevelWarn
			1, // LevelError, actually we don't use it for now
		},
	}
}

func (log *KLog) SetTraceVerbosity(verbosity int) {
	log.verbosities[LevelTrace] = verbosity
}

func (log *KLog) SetDebugVerbosity(verbosity int) {
	log.verbosities[LevelDebug] = verbosity
}

func (log *KLog) SetInfoVerbosity(verbosity int) {
	log.verbosities[LevelInfo] = verbosity
}

func (log *KLog) SetWarnVerbosity(verbosity int) {
	log.verbosities[LevelWarn] = verbosity
}

func (log *KLog) Trace(v ...interface{}) {
	verbosity := klog.Level(log.verbosities[LevelTrace])
	klog.V(verbosity).Info(v...)
}

func (log *KLog) Tracef(format string, v ...interface{}) {
	verbosity := klog.Level(log.verbosities[LevelTrace])
	klog.V(verbosity).Infof(format, v...)
}

func (log *KLog) Debug(v ...interface{}) {
	verbosity := klog.Level(log.verbosities[LevelDebug])
	klog.V(verbosity).Info(v...)
}

func (log *KLog) Debugf(format string, v ...interface{}) {
	verbosity := klog.Level(log.verbosities[LevelDebug])
	klog.V(verbosity).Infof(format, v...)
}

func (log *KLog) Info(v ...interface{}) {
	verbosity := klog.Level(log.verbosities[LevelInfo])
	klog.V(verbosity).Info(v...)
}

func (log *KLog) Infof(format string, v ...interface{}) {
	verbosity := klog.Level(log.verbosities[LevelInfo])
	klog.V(verbosity).Infof(format, v...)
}

func (log *KLog) Warn(v ...interface{}) {
	verbosity := klog.Level(log.verbosities[LevelWarn])
	klog.V(verbosity).Info(v...)
}

func (log *KLog) Warnf(format string, v ...interface{}) {
	verbosity := klog.Level(log.verbosities[LevelWarn])
	klog.V(verbosity).Infof(format, v...)
}

func (log *KLog) Error(v ...interface{}) {
	klog.Error(v...)
}

func (log *KLog) Errorf(format string, v ...interface{}) {
	klog.Errorf(format, v...)
}
