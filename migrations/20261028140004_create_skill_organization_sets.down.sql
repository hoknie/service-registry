DROP TRIGGER skills_forget_in_sets ON skills;

DROP TRIGGER skill_folders_forget_in_sets ON skill_folders;

DROP FUNCTION skill_sets_forget_skill();

DROP FUNCTION skill_sets_forget_folder();

DROP FUNCTION skill_in_sets(uuid, uuid, uuid, uuid[]);

DROP FUNCTION skill_folder_ancestors(uuid);

DROP TABLE skill_organization_sets;
