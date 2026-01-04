import type { User } from '$lib/types/user.type';
import { writable } from 'svelte/store';
import { setLocale } from '$lib/utils/locale.util';

const userStore = writable<User | null>(null);

const setUser = async (user: User) => {
	if (user.locale) {
		await setLocale(user.locale, false);
	}
	userStore.set(user);
};

const clearUser = () => {
	userStore.set(null);
};

export default {
	subscribe: userStore.subscribe,
	setUser,
	clearUser
};
