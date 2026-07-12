UPDATE users
SET role = 'Member'
WHERE role NOT IN ('Admin Gereja', 'Music Director', 'Member');

