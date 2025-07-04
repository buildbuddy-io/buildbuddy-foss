import { grp } from "../../proto/group_ts_proto";
import { user_id } from "../../proto/user_id_ts_proto";
import { user } from "../../proto/user_ts_proto";
import { BuildBuddyServiceRpcName } from "../service/rpc_service";

export class User {
  displayUser: user_id.DisplayUser;
  groups: grp.Group[];
  selectedGroup: grp.Group;
  selectedGroupAccess: user.SelectedGroup.Access;
  allowedRpcs: Set<BuildBuddyServiceRpcName>;
  /** Whether the user has linked their personal GitHub account in order to use
   * GitHub-related features. Linking their personal account is a pre-requisite
   * to manage/view GitHub-related features in our UI, even though many of the features
   * are powered through a GitHub app that is installed at the org level.
   *
   * This is separate from whether the user uses GitHub to login. */
  githubLinked: boolean;
  /** Whether the user is temporarily acting as a member of the selected group. */
  isImpersonating: boolean;
  subdomainGroupID: string;
  codesearchAllowed: boolean;

  constructor(init: Partial<User>) {
    this.displayUser = init.displayUser!;
    this.groups = init.groups!;
    // Note: we use an empty group object here to indicate "no selected group"
    // for convenience, so that selectedGroup is not null. This should not cause
    // issues in practice since the router will redirect to the "create org"
    // page on initial page load if the user is not a part of any groups.
    this.selectedGroup = init.selectedGroup ?? new grp.Group();
    this.selectedGroupAccess = init.selectedGroupAccess ?? user.SelectedGroup.Access.ALLOWED;
    this.allowedRpcs = init.allowedRpcs!;
    this.githubLinked = init.githubLinked!;
    this.isImpersonating = init.isImpersonating!;
    this.subdomainGroupID = init.subdomainGroupID!;
    this.codesearchAllowed = init.codesearchAllowed!;

    // All props are required, but it's a pain in TS to get a type representing
    // "only the fields of User, not the methods". So do a runtime check here.
    for (const prop of Object.getOwnPropertyNames(this) as Array<keyof User>) {
      if (this[prop] === undefined || this[prop] === null) {
        throw new Error(`${prop} property is required`);
      }
    }
  }

  getId() {
    return this.displayUser.userId?.id || "";
  }

  selectedGroupName() {
    if (this.selectedGroup?.name == "DEFAULT_GROUP") return "Organization";
    return this.selectedGroup?.name?.trim();
  }

  canCall(rpc: BuildBuddyServiceRpcName) {
    return this.allowedRpcs.has(rpc);
  }

  canImpersonate() {
    return this.allowedRpcs.has("getInvocationOwner");
  }

  isGroupAdmin() {
    return this.allowedRpcs.has("updateGroup");
  }
}

export function accountName(user: user_id.DisplayUser | undefined | null): string {
  if (!user) {
    return "Unknown User";
  }
  if (user.username) {
    return user.username;
  }
  if (user.email) {
    return user.email;
  }
  return `BuildBuddy User ${user.userId?.id}`;
}
