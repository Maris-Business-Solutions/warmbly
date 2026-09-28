import {
    BellIcon,
    BracesIcon,
    Building2Icon,
    EyeOffIcon,
    MailIcon,
    PhoneIcon,
    ShieldCheckIcon,
    TagsIcon,
    UserIcon,
    UserRoundIcon,
    type LucideIcon,
} from "lucide-react";
import { isCustomTarget } from "../importShared";

const ICONS: Record<string, LucideIcon> = {
    ignore: EyeOffIcon,
    email: MailIcon,
    first_name: UserIcon,
    last_name: UserRoundIcon,
    company: Building2Icon,
    phone: PhoneIcon,
    subscribed: BellIcon,
    categories: TagsIcon,
    verification_status: ShieldCheckIcon,
};

// TargetIcon is the glyph for where a column goes, shared by the picker, the
// mapping table and the contact preview so a field reads the same everywhere.
export default function TargetIcon({ target, className = "w-3 h-3" }: { target: string; className?: string }) {
    const Icon = isCustomTarget(target) ? BracesIcon : ICONS[target] ?? EyeOffIcon;
    return <Icon className={className} aria-hidden="true" />;
}
