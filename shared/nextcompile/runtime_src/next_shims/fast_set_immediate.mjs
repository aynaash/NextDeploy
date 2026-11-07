export function DANGEROUSLY_runPendingImmediatesAfterCurrentTask() {}
export function expectNoPendingImmediates() {}
export function unpatchedSetImmediate() {
    return globalThis.setImmediate;
}

export default {
    DANGEROUSLY_runPendingImmediatesAfterCurrentTask,
    expectNoPendingImmediates,
    unpatchedSetImmediate,
};
