import { useContext, createContext } from "react";

interface ConfirmContextValue {
    /** onCancel runs when the dialog is dismissed without confirming. */
    show: (text: string, onSubmit: () => void | Promise<void>, onCancel?: () => void) => void,
    setLoading: React.Dispatch<React.SetStateAction<boolean>>,
    setShow: React.Dispatch<React.SetStateAction<boolean>>,
}

export const ConfirmContext = createContext<ConfirmContextValue | undefined>(undefined);

export function useConfirm() {
    const c = useContext(ConfirmContext);
    if (!c) {
        throw Error("ConfirmProvider not found")
    }
    return c
}
