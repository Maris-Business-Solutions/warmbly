export default interface Login {
    email: string;
    password: string;
    turnstile: string;
    /** Returned by an earlier confirm with remember_device; skips the emailed code. */
    device_token?: string;
}
