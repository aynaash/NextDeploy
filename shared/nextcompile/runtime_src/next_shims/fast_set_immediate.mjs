export function DANGEROUSLY_runPendingImmediatesAfterCurrentTask() {}
export function expectNoPendingImmediates() {}
export function unpatchedSetImmediate() {
    return typeof globalThis.setImmediate === "function"
        ? globalThis.setImmediate.bind(globalThis)
        : (fn, ...args) => setTimeout(fn, 0, ...args);
}
export class ImmediateTracker {
    hasPendingImmediates() {
        return false;
    }
    scheduleIdleCheck() {}
    onIdle(callback) {
        let cancelled = false;
        const id = setTimeout(() => {
            if (!cancelled) callback();
        }, 0);
        return () => {
            cancelled = true;
            clearTimeout(id);
        };
    }
}
export function runWithNativeImmediateTracking(_tracker, callback, ...args) {
    return callback(...args);
}

export default {
    DANGEROUSLY_runPendingImmediatesAfterCurrentTask,
    expectNoPendingImmediates,
    unpatchedSetImmediate,
    ImmediateTracker,
    runWithNativeImmediateTracking,
};
