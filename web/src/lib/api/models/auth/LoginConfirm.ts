export default interface LoginConfirm {
    session: string;
    code: string;
    /** Ask for a trusted-device token so this browser skips the code next time. */
    remember_device?: boolean;
}
