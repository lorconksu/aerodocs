# Settings

The Settings page is accessible from the sidebar. It has two tabs: **Profile** (available to all users) and **Users** (admins only).

---

## Profile Tab

![Settings Profile](../screenshots/08-settings-profile.png)

### Changing Your Avatar

Click the avatar circle at the top of the profile section. You can upload an image from your computer. The image is stored as part of your account and shown next to your name throughout the interface.

### Changing Your Password

In the **Change Password** section:

1. Enter your **current password**
2. Enter your **new password** (must meet the password policy: at least 8 characters, at least one number, at least one special character)
3. Confirm the new password
4. Click **Update Password**

You will remain logged in after changing your password. All existing sessions (on other devices) will also remain valid until their tokens expire.

---

## Users Tab (Admin Only)

The Users tab lists all accounts registered in AeroDocs. This tab is only visible to admins.

![Settings Users](../screenshots/09-settings-users.png)

### Creating a New User

1. Click **Create User**
2. Fill in the **Username**, **Email**, and **Role** (Admin or Viewer)
3. Click **Create**

AeroDocs generates a temporary password and shows it to you once. Copy it and share it securely with the new user. They will be required to change their password and set up TOTP on their first login.

New users cannot choose their own password during account creation — they must use the temporary password you provide.

### Changing a User's Role

In the user list, click the role badge next to a user's name (it shows "admin" or "viewer"). A dropdown appears letting you switch between roles. The change takes effect immediately — the user's current session will reflect the new role on their next API request.

**Admin** — Full access. Can add and delete servers, manage users, view audit logs, and access all settings.

**Viewer** — Read-only access. Can view the fleet dashboard and server details but cannot make changes. (Specific per-server and per-folder permissions can be configured separately.)

You cannot change your own role.

### Disabling a User's 2FA

If a user has lost access to their authenticator app, an admin can reset their TOTP:

1. Find the user in the list
2. Click the **...** (more options) menu next to their name
3. Choose **Disable 2FA**
4. You will be asked to enter **your own** current TOTP code to confirm the action (this prevents someone with a stolen admin session from locking everyone out)
5. Click **Confirm**

The user's TOTP is cleared. The next time they log in, they will be taken through the TOTP setup flow again before getting access.

### Deleting a User

1. Find the user in the list
2. Click the **...** menu next to their name
3. Choose **Delete User**
4. Confirm the deletion

Deleting a user is permanent and cannot be undone. Their audit log entries are preserved (the entries remain, but the user_id reference becomes orphaned). You cannot delete your own account.
